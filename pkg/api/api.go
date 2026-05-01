package api

import (
	"encoding/json"
	"net/http"
)

// Создаём обработчик HTTP-запросов
func taskHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		addTaskHandler(w, r)
	case http.MethodGet:
		getTaskHandler(w, r)
	case http.MethodPut:
		updateTaskHandler(w, r)
	case http.MethodDelete:
		deleteTaskHandler(w, r)
	}
}

// Инициализируем HTTP-маршруты
func Init() {
	http.HandleFunc("/api/nextdate", nextDayHandler)
	http.HandleFunc("/api/task", taskHandler)
	http.HandleFunc("/api/tasks", tasksHandler)
	http.HandleFunc("/api/task/done", taskDoneHandler)
}

// Отправляем JSON-ответ клиенту
func writeJson(w http.ResponseWriter, data any, ok bool) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(map[bool]int{true: http.StatusOK, false: http.StatusBadRequest}[ok])

	if data == nil {
		w.Write([]byte("null"))
		return
	}

	resp, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Write(resp)
}
