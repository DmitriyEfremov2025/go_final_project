package main

import (
	"log"

	"github.com/DmitriyEfremov2025/go_final_project/pkg/db"
	"github.com/DmitriyEfremov2025/go_final_project/pkg/server"
)

func main() {
	// Подключаемся к базе данных
	err := db.Init("scheduler.db")
	if err != nil {
		log.Printf("Ошибка подключения к базе данных: %v", err)
		return
	}

	// Закрываем соединение с БД
	defer db.Close()

	// Запускаем сервер
	err = server.StartingServer()
	if err != nil {
		log.Printf("Ошибка при запуске сервера: %v", err)
		return
	}
}
