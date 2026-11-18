package transaction

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

// txKey Ключ транзакции в context.Context
type txKey struct{}

// txValue Транзакция в ctx (уровень: транзакция или точка сохранения), пул, на котором она открыта, и
// внешний уровень. Вложенный Exec менеджера на другом подключении открывает свою транзакцию, а не берёт чужую.
// Все уровни идут на одном соединении: вложенные Exec одного уровня выполняются по очереди (sem), иначе
// ROLLBACK TO одной горутины откатил бы записи другой. done — уровень завершён (fn вернулся; транзакция
// зафиксирована или откачена, точка сохранения снята): ctx, переживший Exec, его больше не несёт
type txValue struct {
	pool   gorm.ConnPool
	tx     *gorm.DB
	parent *txValue
	sem    chan struct{}
	done   atomic.Bool
}

func newTxValue(pool gorm.ConnPool, tx *gorm.DB, parent *txValue) *txValue {
	return &txValue{pool: pool, tx: tx, parent: parent, sem: make(chan struct{}, 1)}
}

// finish Помечает уровень завершённым, дождавшись вложенного Exec, который уже идёт на нём
func (v *txValue) finish() {
	v.sem <- struct{}{}
	v.done.Store(true)
	<-v.sem
}

// nested fn точкой сохранения уровня v, по очереди с другими вложенными Exec этого уровня. false — уровень
// завершился, пока ждали очереди: fn не вызывалась
func (v *txValue) nested(ctx context.Context, fn func(ctx context.Context, tx *gorm.DB) error) (bool, error) {
	select {
	case v.sem <- struct{}{}:
	case <-ctx.Done():
		return true, ctx.Err()
	}

	defer func() { <-v.sem }()

	if v.done.Load() {
		return false, nil
	}

	return true, v.tx.WithContext(ctx).Transaction(func(sp *gorm.DB) error {
		child := newTxValue(v.pool, sp, v)
		defer child.finish()

		return fn(context.WithValue(ctx, txKey{}, child), sp)
	})
}

// active Открытый уровень транзакции из ctx: завершённая точка сохранения — её внешний уровень; nil — в ctx
// нет транзакции или она завершена
func active(ctx context.Context) *txValue {
	v, _ := ctx.Value(txKey{}).(*txValue)
	for v != nil && v.done.Load() {
		v = v.parent
	}

	return v
}

// run Транзакция для fn. В ctx уже есть открытая транзакция того же подключения (вложенный Exec) — fn идёт в
// ней точкой сохранения (SAVEPOINT): ошибка fn откатывает только свои изменения, фиксирует всё внешняя
// транзакция; timeout и opts не применяются — действует внешняя. Вложенные Exec с одним ctx из разных горутин
// выполняются по очереди, целиком: соединение у транзакции одно. Иначе (и если транзакция в ctx уже
// зафиксирована или откачена) — новая транзакция с timeout (≤ 0 — без своего таймаута) и opts. fn получает ctx
// с транзакцией (FromContext)
func run(ctx context.Context, client *gorm.DB, timeout time.Duration, opts *sql.TxOptions, fn func(ctx context.Context, tx *gorm.DB) error) error {
	pool := client.Statement.ConnPool

	for v := active(ctx); v != nil && v.pool == pool; v = active(ctx) {
		if ran, err := v.nested(ctx, fn); ran {
			return err
		}
	}

	if timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	txOpts := []*sql.TxOptions{}
	if opts != nil {
		txOpts = append(txOpts, opts)
	}

	return client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		v := newTxValue(pool, tx, nil)
		defer v.finish()

		return fn(context.WithValue(ctx, txKey{}, v), tx)
	}, txOpts...)
}

// FromContext Открытая транзакция из ctx (внутри Exec), если она на подключении client; иначе client (и после
// фиксации или отката транзакции). Для кода, который получает ctx, но не репозиторий из Exec:
// FromContext(ctx, r.client).WithContext(ctx)
func FromContext(ctx context.Context, client *gorm.DB) *gorm.DB {
	if v := active(ctx); v != nil && v.pool == client.Statement.ConnPool {
		return v.tx
	}

	return client
}

// InTransaction В ctx есть открытая транзакция Exec (любого подключения)
func InTransaction(ctx context.Context) bool {
	return active(ctx) != nil
}

// WithoutTransaction ctx без транзакции Exec (значения, дедлайн и отмена — те же): Exec с ним открывает
// новую независимую транзакцию — например, журнал, который фиксируется и при откате внешней, или
// параллельная работа в горутинах (вложенные Exec с одним ctx идут по очереди)
func WithoutTransaction(ctx context.Context) context.Context {
	return context.WithValue(ctx, txKey{}, (*txValue)(nil))
}

// sqlStater Ошибка драйвера с SQLSTATE (pgconn.PgError и подобные)
type sqlStater interface {
	SQLState() string
}

// IsRetryable Ошибка — сбой сериализации (SQLSTATE 40001) или взаимоблокировка (40P01): транзакцию можно
// повторить целиком
func IsRetryable(err error) bool {
	var s sqlStater
	if !errors.As(err, &s) {
		return false
	}

	code := s.SQLState()

	return code == "40001" || code == "40P01"
}

// Retry Вызывает fn до attempts раз (меньше 1 — один раз), пока ошибка IsRetryable; между попытками —
// пауза 20 мс × номер попытки, отмена ctx прерывает ожидание. Внутри уже открытой транзакции (InTransaction)
// любого подключения не повторяет: внешняя транзакция после такой ошибки прервана, повторять надо её. Подключения
// fn Retry не знает: если в ctx транзакция другой БД, а fn открывает свою (Exec на другом подключении), — передать
// WithoutTransaction(ctx), иначе повтора не будет.
// Пример: transaction.Retry(ctx, 3, func(ctx context.Context) error {
// return m.ExecWithOptions(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable}, fn) })
func Retry(ctx context.Context, attempts int, fn func(ctx context.Context) error) error {
	if attempts < 1 || InTransaction(ctx) {
		attempts = 1
	}

	var err error

	for i := 1; i <= attempts; i++ {
		if err = fn(ctx); err == nil || !IsRetryable(err) || i == attempts {
			return err
		}

		timer := time.NewTimer(time.Duration(i) * 20 * time.Millisecond)

		select {
		case <-ctx.Done():
			timer.Stop()

			return errors.Join(err, ctx.Err())
		case <-timer.C:
		}
	}

	return err
}
