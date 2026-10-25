package handlers

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	// failWindow — окно, в котором считаются неудачные попытки.
	failWindow = 15 * time.Minute

	// accountLockThreshold — жёсткая граница на один аккаунт (V2.2.1): пять
	// неверных паролей закрывают вход на этот email.
	accountLockThreshold = 5

	// ipLockThreshold — мягкая граница на адрес. Она заведомо выше account:
	// адрес провайдера или кампусной сети общий, и по одной попытке от сотни
	// людей блокировка по IP наказала бы всех сразу, включая тех, кого хотел
	// защитить злоумышленник.
	ipLockThreshold = 25

	lockDuration = 15 * time.Minute
)

// Limiter — защита от подбора паролей и перебора email (V2.2.1, WSTG-ATHN-04/07).
// Счётчики живут в памяти процесса: как и с картой сессий, это компромисс
// одиночного учебной инстанса. За общим балансировщиком нужен общий стор
// (Redis и т. п.), иначе каждый сервер считает свои попытки.
type Limiter struct {
	mu     sync.Mutex
	events map[string]*attempt
}

type attempt struct {
	count   int
	blocked time.Time
	last    time.Time
}

func NewLimiter() *Limiter {
	return &Limiter{events: make(map[string]*attempt)}
}

// Allowed говорит, можно ли принимать попытку прямо сейчас. Пустой wait —
// блокировка не действует.
func (l *Limiter) Allowed(key string) (wait time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	event, ok := l.events[key]
	if !ok {
		return 0
	}

	now := time.Now()
	if event.blocked.After(now) {
		return event.blocked.Sub(now)
	}
	// окно прошло — накопление начинается заново
	if now.Sub(event.last) > failWindow {
		delete(l.events, key)
	}
	return 0
}

// Fail учитывает неудачную попытку и возвращает true, если включилась
// блокировка. Порог передаёт вызывающий код: у аккаунта и у адреса они разные.
func (l *Limiter) Fail(key string, threshold int) (blocked bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweepLocked()

	now := time.Now()
	event := l.events[key]
	if event == nil {
		event = &attempt{last: now}
		l.events[key] = event
	}
	if now.Sub(event.last) > failWindow {
		event.count = 0
	}
	event.last = now
	event.count++

	if event.count >= threshold {
		event.blocked = now.Add(lockDuration)
		return true
	}
	return false
}

// Success сбрасывает счётчик: успешный вход не должен наказывать за
// опечатку в предыдущей попытке.
func (l *Limiter) Success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.events, key)
}

// Quota разрешает не более limit однотипных действий на ключ внутри окна.
// Нужен там, где «неудача» — не ошибка пользователя, а способ злоупотребления:
// массовая саморегистрация не генерирует неверных паролей, поэтому обычный
// счётчик попыток её не ловит (Cheat Sheet Authentication: ограничение на
// создание аккаунтов с одного адреса).
// Ключ для квоты отдаётся отдельным префиксом: те же строки в events значит
// считали бы и попытки входа.
func (l *Limiter) Quota(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepLocked()

	now := time.Now()
	event := l.events[key]
	if event == nil || now.Sub(event.last) > window {
		l.events[key] = &attempt{count: 1, last: now}
		return true
	}

	event.last = now
	event.count++
	return event.count <= limit
}

// sweepLocked выкидывает протухшие ключи, чтобы map не рос бесконечно
// за счёт одноразовых IP. Вызывается под замком, когда карта уже велика.
func (l *Limiter) sweepLocked() {
	if len(l.events) < 4096 {
		return
	}
	for key, event := range l.events {
		if time.Since(event.last) > failWindow && event.blocked.Before(time.Now()) {
			delete(l.events, key)
		}
	}
}

// clientIP берёт адрес соединения. X-Forwarded-For сознательно не читается:
// заголовок подделывается клиентом, и лимитер можно было бы обойти или
// направить на чужой адрес. Доверять ему можно только известному прокси.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
