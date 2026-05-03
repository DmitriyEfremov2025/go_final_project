package db

import (
	"database/sql"
	"fmt"
	"time"
)

const DateFormat, incorrectFormat = "20060102", "02.01.2006"
const TaskLimit = 50

// Создаём структуру Task с нужными полями
type Task struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

// Добавляем функцию добавления задачи в БД
// Возвращаем идентификатор добавленной записи
func AddTask(task *Task) (int64, error) {
	var id int64
	query := `INSERT INTO scheduler (date, title, comment, repeat) VALUES (:date, :title, :comment, :repeat)`
	res, err := db.Exec(query,
		sql.Named("date", task.Date),
		sql.Named("title", task.Title),
		sql.Named("comment", task.Comment),
		sql.Named("repeat", task.Repeat))
	if err == nil {
		id, err = res.LastInsertId()
	}
	return id, err
}

// Создаём функцию для поиска задачи по идентификатору
// Ответом будет искомая задача или ошибка
func GetTask(id string) (*Task, error) {
	if id == "" {
		return nil, fmt.Errorf("ID not specified")
	}
	task := &Task{}
	query := `SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?`
	err := db.QueryRow(query, id).Scan(
		&task.ID,
		&task.Date,
		&task.Title,
		&task.Comment,
		&task.Repeat,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found")
		}
		return nil, err
	}
	return task, nil
}

// Создаём функцию для редактирования задачи
func UpdateTask(task *Task) error {
	query := `UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat = ? WHERE id = ?`
	res, err := db.Exec(
		query,
		task.Date,
		task.Title,
		task.Comment,
		task.Repeat,
		task.ID,
	)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

// Вспомогательная функция, которая скрывает всю работу с rows для функции Tasks
func fetchTasks(query string, args ...any) ([]*Task, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error executing database query: %w", err)
	}
	defer rows.Close()

	tasks := make([]*Task, 0)

	for rows.Next() {
		task := &Task{}
		err := rows.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
		if err != nil {
			return nil, fmt.Errorf("error scanning database row: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error while iterating over query results: %w", err)
	}

	return tasks, nil
}

// Функция получет задачи с фильтрацией по поиску и дате
func Tasks(limit int, search string) ([]*Task, error) {
	// Проверяем формат даты DD.MM.YYYY
	if isDateFormat(search) {
		dateStr := convertToDBDate(search)
		query := `SELECT id, date, title, comment, repeat FROM scheduler WHERE date = ? ORDER BY date ASC LIMIT ?`
		return fetchTasks(query, dateStr, limit)
	}

	// Выполняем поиск по подстроке в заголовке или комментарии
	if search != "" {
		searchPattern := "%" + search + "%"
		query := `SELECT id, date, title, comment, repeat FROM scheduler WHERE title LIKE ? OR comment LIKE ? ORDER BY date ASC LIMIT ?`
		return fetchTasks(query, searchPattern, searchPattern, limit)
	}

	// Показываем ближайшие задачи, если поиска не было
	query := `SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date ASC LIMIT ?`
	return fetchTasks(query, limit)
}

// Проверяет, что строка соответствует формату DD.MM.YYYY
func isDateFormat(s string) bool {
	_, err := time.Parse(incorrectFormat, s)
	return err == nil
}

// Преобразуем дату из DD.MM.YYYY в наш формат
func convertToDBDate(dateStr string) string {
	t, _ := time.Parse(incorrectFormat, dateStr)
	return t.Format(DateFormat)
}

// Создаём фукнцию удаления задачи по идентификатору
func DeleteTask(id string) error {
	query := `DELETE FROM scheduler WHERE id = ?`
	res, err := db.Exec(query, id)
	if err != nil {
		return err
	}

	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

// Создаём функцию обновления только даты задачи по идентификатору
func UpdateDate(nextDate, id string) error {
	query := `UPDATE scheduler SET date = ? WHERE id = ?`
	res, err := db.Exec(query, nextDate, id)
	if err != nil {
		return err
	}

	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}
