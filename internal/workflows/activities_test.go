package workflows

import (
	"testing"
	"time"
)

func TestNotificationText_Water(t *testing.T) {
	got := NotificationText("water", "shutdown", "г. Тирасполь, ул. Ленина, д. 1, 2",
		time.Date(2025, 12, 2, 9, 0, 0, 0, time.UTC), "Плановые работы")
	want := "💧 Отключение услуги водоснабжения по адресу «г. Тирасполь, ул. Ленина, д. 1, 2» с 02-12-2025 09:00\n\nПлановые работы"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestNotificationText_ElectricityResume(t *testing.T) {
	got := NotificationText("electricity", "resume", "адрес",
		time.Date(2025, 1, 5, 18, 30, 0, 0, time.UTC), "d")
	want := "⚡️ Возобновление услуги электроснабжения по адресу «адрес» с 05-01-2025 18:30\n\nd"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestNotificationText_UnknownSupplierAndEvent(t *testing.T) {
	got := NotificationText("gas", "other", "a",
		time.Date(2025, 1, 5, 18, 30, 0, 0, time.UTC), "d")
	want := "  услуги  по адресу «a» с 05-01-2025 18:30\n\nd"
	if got != want { // empty emoji/name/description — like switch default in Java
		t.Fatalf("got %q", got)
	}
}
