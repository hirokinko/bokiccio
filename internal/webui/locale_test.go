package webui

import (
	"reflect"
	"strings"
	"testing"
)

func TestMessagesAreComplete(t *testing.T) {
	for _, item := range []struct {
		name     string
		messages messages
	}{
		{name: "ja", messages: japaneseMessages()},
		{name: "en", messages: englishMessages()},
	} {
		value := reflect.ValueOf(item.messages)
		typ := value.Type()
		for i := range value.NumField() {
			field := value.Field(i)
			fieldName := typ.Field(i).Name
			switch field.Kind() {
			case reflect.String:
				if strings.TrimSpace(field.String()) == "" {
					t.Errorf("%s.%s is empty", item.name, fieldName)
				}
			case reflect.Func:
				if field.IsNil() {
					t.Errorf("%s.%s is nil", item.name, fieldName)
				}
			default:
				t.Errorf("%s.%s has unsupported kind %s", item.name, fieldName, field.Kind())
			}
		}
		for name, text := range map[string]string{
			"EntriesShown":            item.messages.EntriesShown(2),
			"CurrentCandidateEyebrow": item.messages.CurrentCandidateEyebrow(1),
			"RevisionHeader":          item.messages.RevisionHeader(1),
			"BaseRevisionSummary":     item.messages.BaseRevisionSummary(0, "2026-08-11T10:00:00Z", true),
			"CurrentApprovalSummary":  item.messages.CurrentApprovalSummary(1, "2026-08-11T10:00:00Z"),
			"ApprovalSummary":         item.messages.ApprovalSummary(1, 1, "2026-08-11T10:00:00Z"),
			"RecordStatusSummary":     item.messages.RecordStatusSummary(1, "imported"),
			"IdentitySummary":         item.messages.IdentitySummary("source", 1, "digest"),
		} {
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s.%s returned empty text", item.name, name)
			}
		}
	}
}

func TestLocalizedPath(t *testing.T) {
	for _, test := range []struct {
		name      string
		locale    locale
		localPath string
		want      string
	}{
		{name: "ja index", locale: localeJA, localPath: "/", want: "/"},
		{name: "ja detail", locale: localeJA, localPath: "/entries/entry-1", want: "/entries/entry-1"},
		{name: "en index", locale: localeEN, localPath: "/", want: "/en/"},
		{name: "en detail", locale: localeEN, localPath: "/entries/entry-1", want: "/en/entries/entry-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := localizedPath(test.locale, test.localPath); got != test.want {
				t.Fatalf("localizedPath() = %q, want %q", got, test.want)
			}
		})
	}
}
