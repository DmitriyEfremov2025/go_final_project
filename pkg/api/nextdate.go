package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Определяем константу DateFormat для формата даты в виде строки.
const DateFormat = "20060102"

// Проверяем, что дата date позже, чем дата now
func afterNow(date, now time.Time) bool {
	d1 := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	d2 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return d1.After(d2)
}

// Функция NextDate вычисляет следующую дату согласно правил повторения.
// Функцию принимает текущее время, начальную дату и правило повторения. Возвращает следующую дату или ошибку.
func NextDate(now time.Time, dstart string, repeat string) (string, error) {
	if repeat == "" { // проверка наличие пустой строки в параметре repeat
		return "", errors.New("empty line")
	}
	// Проверка преобразования dstart
	dateStart, err := time.Parse(DateFormat, dstart)
	if err != nil {
		return "", errors.New("invalid time format")
	}

	// проверка формата repeat
	parts := strings.Split(repeat, " ")
	ruleType := parts[0]

	switch ruleType {
	case "d":
		if len(parts) != 2 {
			return "", errors.New("invalid format")
		}
		interval, err := strconv.Atoi(parts[1])
		if err != nil || interval <= 0 {
			return "", errors.New("invalid interval")
		}
		if interval > 400 {
			return "", errors.New("the interval exceeds 400 days")
		}

		// Присваиваем переменной date значение начальной даты.
		date := dateStart
		// Запускаем бесконечный цикл для поиска следующей даты.
		for {
			date = date.AddDate(0, 0, interval)
			if afterNow(date, now) {
				break
			}
		}
		// Преобразуем и возвращаем дату в формате "20060102"
		return date.Format(DateFormat), nil

	case "y": // Если тип правила - "y" (годы)
		if len(parts) != 1 {
			return "", errors.New("invalid format")
		}

		date := dateStart
		for {
			date = date.AddDate(1, 0, 0)
			if afterNow(date, now) {
				break
			}
		}
		return date.Format(DateFormat), nil

	default:
		return "", fmt.Errorf("unsupported repeat rule: %s", ruleType)
	}
}

func nextDayHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем Get-параметры
	nowStr := r.FormValue("now")
	dateStr := r.FormValue("date")
	repeat := r.FormValue("repeat")

	// Проверяем параметр date
	if dateStr == "" {
		http.Error(w, "the 'date' parameter is required", http.StatusBadRequest)
		return
	}
	// Проверяем параметр repeat
	if repeat == "" {
		http.Error(w, "the 'repeat' parameter is required", http.StatusBadRequest)
		return
	}

	// Если параметр now передан, то парсим его в time.Time
	var now time.Time
	if nowStr != "" {
		var err error
		now, err = time.Parse(DateFormat, nowStr)
		if err != nil {
			http.Error(w, "invalid date format", http.StatusBadRequest)
			return
		}
	} else {
		now = time.Now()
	}

	// Вызываем функцию NextDate для расчёта следующей даты
	nextDate, err := NextDate(now, dateStr, repeat)
	if err != nil {
		log.Printf("Ошибка при расчете следующей даты: %v (date: %s, repeat: %s)", err, dateStr, repeat)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Выводим ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprintf(w, "%s", nextDate); err != nil {
		log.Printf("Ошибка при записи текстового ответа: %v", err)
	}
}
