package helpers

import "strings"

// likeEscaper Экранирует спецсимволы LIKE обратной косой чертой
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike Строка для LIKE/ILIKE без подстановочных символов: `%`, `_` и `\` экранированы `\` (экранирующий
// символ LIKE по умолчанию в Postgres и MySQL; в SQLite — добавить ESCAPE '\'). Значение по-прежнему передавать
// параметром: Where("name ILIKE ?", "%"+helpers.EscapeLike(q)+"%")
func EscapeLike(s string) string {
	return likeEscaper.Replace(s)
}
