package api

import (
	"net/http"

	"github.com/DmitriyEfremov2025/go_final_project/pkg/db"
)

// Создаём структуру для формирования JSON с параметром task
type responseTask struct {
	Tasks []*db.Task `json:"tasks"`
}

// Создаём хендлер для получения списка задачс параметрами поиска
// Ответом возвращается JSON со списком задач или ошибка
func tasksHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод запроса
	if r.Method != http.MethodGet {
		writeJson(w, map[string]string{"error": "method not allowed"}, false)
		return
	}

	// Получаем параметр search из строки запроса
	search := r.URL.Query().Get("search")

	// Получаем задачи с учётом поиска
	tasks, err := db.Tasks(db.TaskLimit, search)
	if err != nil {
		writeJson(w, map[string]string{"error": err.Error()}, false)
		return
	}

	writeJson(w, responseTask{
		Tasks: tasks}, true)
}
