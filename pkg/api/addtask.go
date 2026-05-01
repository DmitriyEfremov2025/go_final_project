package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/DmitriyEfremov2025/go_final_project/pkg/db"
)

// Создаём структуру для ответа с ошибкой
type responseErr struct {
	Error string `json:"error"`
}

// Создаём структуру, которая содержит в себе идентификатор созданной задачи
type responseAddTask struct {
	ID string `json:"id"`
}

// Проверяем на корректность значение task.Date
func checkDate(task *db.Task) error {
	now := time.Now()

	// Если дата пустая, устанавливаем сегодняшнее число
	if task.Date == "" {
		task.Date = now.Format(DateFormat)
		return nil
	}

	// Проверяем корректность формата даты
	t, err := time.Parse(DateFormat, task.Date)
	if err != nil {
		return errors.New("the date is presented in a format other than 20060102")
	}

	// Если дата задачи меньше сегодняшней, корректируем её
	if now.After(t) && now.Format(DateFormat) != task.Date {
		if len(task.Repeat) == 0 {
			// Если правила повторения нет, то берём сегодняшнее число
			task.Date = now.Format(DateFormat)
			return nil
		}
		// В противном случае, берём следующую дату
		next, err := NextDate(now, task.Date, task.Repeat)
		if err != nil {
			return errors.New("the repetition rule is specified in the wrong format")
		}
		task.Date = next
	}
	return nil
}

// Создаём хендлер для добавления задачи
// Ожидается JSON с параметрами задачи
// Возвращается ответ с идентификатор задачи или ошибку
func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	var task db.Task
	var response responseAddTask

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, responseErr{Error: "Ошибка при чтении тела запроса"}, false)
		return
	}

	// Десериализация JSON
	if err = json.Unmarshal(body, &task); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, responseErr{Error: "Ошибка десериализации JSON"}, false)
		return
	}

	// Проверка обязательного поля title
	if task.Title == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, responseErr{Error: "Название задачи не указано"}, false)
		return
	}

	// Проверка даты
	err = checkDate(&task)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, responseErr{Error: err.Error()}, false)
		return
	}

	// Добавление задачи в базу данных
	id, err := db.AddTask(&task)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJson(w, responseErr{Error: "Ошибка при добавлении задачи в базу данных"}, false)
		return
	}

	w.WriteHeader(http.StatusOK)
	response.ID = strconv.FormatInt(id, 10)
	writeJson(w, response, true)
}
