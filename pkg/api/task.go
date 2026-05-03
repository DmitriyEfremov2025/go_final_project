package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/DmitriyEfremov2025/go_final_project/pkg/db"
)

// Создаём хендлер для возвращения задачи найденной по идетификатору
func getTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем параметр id из URL
	id := r.URL.Query().Get("id")

	// Проверяем, что идентификатор указан
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, map[string]string{"error": "ID not specified"}, false)
		return
	}

	// Проверяем идентификатор на корректность
	if _, err := strconv.Atoi(id); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, map[string]string{"error": "invalid task ID"}, false)
		return
	}

	// Получаем задачу из БД
	task, err := db.GetTask(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJson(w, map[string]string{"error": err.Error()}, false)
		return
	}

	// Возвращаем задачу в формате JSON
	writeJson(w, task, true)
}

// Создаём хендлер для редактирования задачи
func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	var task db.Task

	// Декодируем JSON из тела запроса
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, map[string]string{"error": "invalid JSON format"}, false)
		return
	}

	// Базовая валидация обязательных полей
	if task.ID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, map[string]string{"error": "task ID not specified"}, false)
		return
	}
	if task.Date == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, map[string]string{"error": "the task completion date is not specified."}, false)
		return
	}
	if task.Title == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, map[string]string{"error": "the task title is not specified"}, false)
		return
	}
	// Проверяем корректность формата даты
	err = checkDate(&task)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, map[string]string{"error": err.Error()}, false)
		return
	}

	// Обновляем задачу в БД
	err = db.UpdateTask(&task)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJson(w, map[string]string{"error": err.Error()}, false)
		return
	}

	// Успешное обновление — возвращаем пустой JSON
	writeJson(w, map[string]interface{}{}, true)
}

// Создаём хендлер для удаления задачи
func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем параметр id из URL
	id := r.URL.Query().Get("id")

	// Проверяем, что ID указан
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, map[string]string{"error": "ID not specified"}, false)
		return
	}

	// Проверяем идентификатор на корректность
	if _, err := strconv.Atoi(id); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, map[string]string{"error": "invalid task ID"}, false)
		return
	}

	// Удаляем задачу из БД
	err := db.DeleteTask(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJson(w, map[string]string{"error": err.Error()}, false)
		return
	}

	// Если удаление успешное, то возвращаем пустой JSON
	writeJson(w, map[string]interface{}{}, true)
}

// Создаём хендлер для отметки выполненной задачи
func taskDoneHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод на корректность
	if r.Method != http.MethodPost {
		http.Error(w, "the method is not supported", http.StatusMethodNotAllowed)
		return
	}

	// Получаем идентификатор задачи
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, map[string]string{"error": "ID not specified"}, false)
		return
	}

	// Проверяем идентификатор на корректность
	if _, err := strconv.Atoi(id); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJson(w, map[string]string{"error": "invalid task ID"}, false)
		return
	}

	// Получаем задачу из БД по идентификатору
	task, err := db.GetTask(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJson(w, map[string]string{"error": err.Error()}, false)
		return
	}

	// Проверяем, указана ли периодичность
	if task.Repeat == "" {
		err = db.DeleteTask(id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJson(w, map[string]string{"error": err.Error()}, false)
			return
		}
		writeJson(w, map[string]interface{}{}, true)
		return
	} else {
		nextDate, err := NextDate(time.Now(), task.Date, task.Repeat)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJson(w, map[string]string{"error": err.Error()}, false)
			return
		}

		// Обновляем дату в задаче
		task.Date = nextDate

		// Сохраняем обновлённую задачу в БД
		err = db.UpdateTask(task)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJson(w, map[string]string{"error": err.Error()}, false)
			return
		}
		writeJson(w, map[string]interface{}{}, true)
	}
}
