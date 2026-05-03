package db // Пакет для работы с базой данных.

import (
	"database/sql"
	"os"

	_ "modernc.org/sqlite"
)

// Объявляем глобальную переменную db.
var db *sql.DB

// Определяем строковую константу с SQL командами
const schema = `CREATE TABLE scheduler (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date CHAR(8) NOT NULL DEFAULT "",
    title VARCHAR(255) NOT NULL DEFAULT "",
    comment TEXT NOT NULL DEFAULT "",
    repeat VARCHAR(128) NOT NULL DEFAULT ""
);

CREATE INDEX scheduler_date ON scheduler (date);` // Создаем индекс на поле date таблицы scheduler для ускорения поиска по этому полю.

// Объявляем функцию Init, которая проверяет существование файла переданного в dbFile, открывает базу данных и при необходимости создаёт таблицу.
func Init(dbFile string) error {
	// Получаем значение переменной окружения TODO_DBFILE, которая может содержать путь к файлу базы данных.
	envPath := os.Getenv("TODO_DBFILE")
	if envPath != "" { // Проверяем, задано ли значение переменной окружения.
		dbFile = envPath // Если задано, присваиваем переменной dbFile значение из переменной окружения.
	}
	_, err := os.Stat(dbFile) // Проверяем существование файла базы данных по указанному пути.
	var install bool          // Объявляем переменную install для определения, нужно ли создавать таблицу.
	if err != nil {           // Если произошла ошибка (например, файл не существует),
		install = true // устанавливаем install в true, что указывает на необходимость создания таблицы.
	}

	dbConnection, err := sql.Open("sqlite", dbFile) // Открываем соединение с базой данных SQLite по указанному пути.
	if err != nil {
		return err
	}

	db = dbConnection // Присваиваем глобальной переменной db значение открытого соединения.

	// Создание таблицы при необходимости
	if install {
		_, err = db.Exec(schema) // Выполняем SQL-команды из константы schema для создания таблицы и индекса.
		if err != nil {
			db.Close() // Закрываем соединение с базой данных в случае ошибки.
			db = nil   // Устанавливаем глобальную переменную db в nil, чтобы указать, что соединение закрыто.
			return err
		}
	}

	return nil // Возвращаем nil, что означает успешное выполнение функции без ошибок.
}

// Закрываем соединение с БД.
func Close() error {
	if db == nil { // Проверяем, было ли открыто соединение (если db равно nil).
		return nil
	}
	err := db.Close() // Закрываем соединение с базой данных и сохраняем возможную ошибку в переменной err.
	db = nil          // Устанавливаем глобальную переменную db в nil, чтобы указать, что соединение закрыто.
	return err
}
