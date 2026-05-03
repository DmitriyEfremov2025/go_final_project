package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/DmitriyEfremov2025/go_final_project/pkg/api"
)

// Объявляем функцию StartingServer, которая запускает HTTP-сервер и возвращает ошибку, если она произошла.
func StartingServer() error {
	webDir := "./web" // Определяем переменную webDir, указывающую на директорию с фронтенд-файлами.

	// Получаем значение переменной TODO_PORT
	portStr := os.Getenv("TODO_PORT")

	defaultPort := 7540 // Определяем порт по умолчанию, если переменная окружения не задана.
	var port int        // Объявляем переменную port для хранения номера порта.

	// Преобразуем значение переменной в число
	if portStr != "" { // Проверяем, задано ли значение переменной TODO_PORT.
		parsedPort, err := strconv.Atoi(portStr) // Преобразуем строку в целое число.
		if err != nil {
			return fmt.Errorf("error: TODO_PORT value '%s'. Use a number between 1 and 65535.", portStr)
		}

		// Проверяем допустимый диапазон
		if parsedPort < 1 || parsedPort > 65535 {
			return fmt.Errorf("error: Port %d is not in the valid range (1-65535)", parsedPort)
		}

		port = parsedPort // Если все проверки пройдены, присваиваем переменной port значение parsedPort.
	} else {
		port = defaultPort // Если переменная TODO_PORT не задана, используем порт по умолчанию.
	}

	// Формируем строку listenAddress, которая содержит адрес для прослушивания сервера.
	listenAddress := fmt.Sprintf(":%d", port)

	// Инициализируем API приложения с помощью функции Init из пакета api.
	api.Init()

	// Создаем обработчик файл-сервера для обслуживания статических файлов из директории webDir.
	fileServer := http.FileServer(http.Dir(webDir))

	// Устанавливаем обработчик для корневого пути, который будет использовать созданный файл-сервер.
	http.Handle("/", http.StripPrefix("/", fileServer))

	// Получаем значение переменной окружения TODO_DBFILE, которая может содержать путь к файлу базы данных.
	envPath := os.Getenv("TODO_DBFILE")

	log.Printf("Сервер запущен на порту%s", listenAddress)
	log.Printf("Переменная TODO_PORT:%s (используемый порт: %d)", portStr, port)
	if envPath != "" { // Проверяем, задан ли путь к базе данных через переменную окружения.
		log.Printf("Путь к базе данных переопределен параметром TODO_DBFILE: '%s'", envPath)
	} else {
		log.Printf("Используется путь к базе данных по умолчанию: 'scheduler.db'")
	}

	return http.ListenAndServe(listenAddress, nil) // Запускаем HTTP-сервер на указанном адресе и возвращаем ошибку, если она произошла.
}
