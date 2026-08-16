// Package store — персистентность (pgx). Схема таблиц не меняется
// относительно Java-версии; маппинги повторяют JPA-entity 1:1.
package store

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"events-workflow/internal/domain"
)

func StrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type TelegramMessageEntity struct {
	ID            int64
	ChatID        int64
	Text          string
	Date          time.Time
	ServiceName   *string
	FromID        *int64
	FromName      *string
	ReplyToID     *int64
	ReplyToChatID *int64
	IncidentID    *uuid.UUID
	Context       map[string]string
}

func TelegramMessageEntityFrom(m domain.TelegramMessage, incidentID uuid.UUID) *TelegramMessageEntity {
	e := &TelegramMessageEntity{
		ID: m.ID, ChatID: m.ChatID, Text: m.Text, Date: m.Date.Time,
		ServiceName: StrPtr(m.ServiceName), Context: m.Context,
		IncidentID: &incidentID,
	}
	if m.From != nil {
		e.FromID, e.FromName = &m.From.ID, StrPtr(m.From.Name)
	}
	if m.ReplyTo != nil {
		e.ReplyToID, e.ReplyToChatID = &m.ReplyTo.ID, &m.ReplyTo.ChatID
	}
	return e
}

func (e *TelegramMessageEntity) ToDomain() domain.TelegramMessage {
	var from *domain.User
	if e.FromID != nil || e.FromName != nil {
		var id int64
		if e.FromID != nil {
			id = *e.FromID
		}
		from = &domain.User{ID: id, Name: deref(e.FromName)}
	}
	var reply *domain.MessageReference
	if e.ReplyToID != nil || e.ReplyToChatID != nil {
		reply = &domain.MessageReference{}
		if e.ReplyToID != nil {
			reply.ID = *e.ReplyToID
		}
		if e.ReplyToChatID != nil {
			reply.ChatID = *e.ReplyToChatID
		}
	}
	supplier := deref(e.ServiceName)
	if supplier == "" && e.Context != nil {
		supplier = e.Context["supplier"]
	}
	return domain.TelegramMessage{
		ID: e.ID, ChatID: e.ChatID, From: from, Text: e.Text,
		Date: domain.LocalDateTime{Time: e.Date}, ReplyTo: reply,
		ServiceName: supplier, Context: e.Context,
	}
}

type TranscribeEntity struct {
	ID           int64
	ChatID       int64
	Organization *string
	Description  *string
	Event        *string
	EventStart   *time.Time
	EventStop    *time.Time
}

func TranscribeEntityFrom(pm domain.ParsedMessage) *TranscribeEntity {
	tr := pm.MessageTranscription
	e := &TranscribeEntity{
		ID: pm.OriginalMessage.ID, ChatID: pm.OriginalMessage.ChatID,
		Organization: StrPtr(tr.Organization),
		Description:  StrPtr(tr.ShortDescription),
		Event:        StrPtr(tr.Event),
	}
	if tr.EventStart != nil {
		t := tr.EventStart.Time
		e.EventStart = &t
	}
	if tr.EventStop != nil {
		t := tr.EventStop.Time
		e.EventStop = &t
	}
	return e
}

type AddressEntity struct {
	ID                 uuid.UUID
	MessageID          int64
	ChatID             int64
	CityOriginal       *string
	StreetOriginal     *string
	StreetTypeOriginal *string
	HouseNumbers       *string
	HouseRanges        *string
	RegionName         *string
	RegionKladr        *string
	RegionType         *string
	CityName           *string
	CityKladr          *string
	CityType           *string
	StreetName         *string
	StreetKladr        *string
	StreetType         *string
}

func AddressEntityFrom(a domain.Address, ref domain.MessageReference) *AddressEntity {
	e := &AddressEntity{
		ID: a.ID, MessageID: ref.ID, ChatID: ref.ChatID,
		CityOriginal:       StrPtr(a.City),
		StreetOriginal:     StrPtr(a.Street),
		StreetTypeOriginal: StrPtr(a.StreetType),
	}
	if a.House != nil {
		if len(a.House.Numbers) > 0 {
			e.HouseNumbers = StrPtr(strings.Join(a.House.Numbers, ","))
		}
		if len(a.House.Ranges) > 0 {
			ranges := make([]string, len(a.House.Ranges))
			for i, r := range a.House.Ranges {
				ranges[i] = strings.Join(r, "-")
			}
			e.HouseRanges = StrPtr(strings.Join(ranges, ";"))
		}
	}
	return e
}

// Houses — список домов для события (порт houses() из MessageProcessActivity).
func (e *AddressEntity) Houses() []string {
	var parts []string
	if s := deref(e.HouseNumbers); strings.TrimSpace(s) != "" {
		parts = append(parts, strings.Split(s, ",")...)
	}
	if s := deref(e.HouseRanges); strings.TrimSpace(s) != "" {
		parts = append(parts, strings.Split(s, ";")...)
	}
	return parts
}

// FormatHouses — «д. 1, 2, 10-20» для текста уведомления (порт formatHouses()).
func (e *AddressEntity) FormatHouses() string {
	parts := e.Houses()
	if len(parts) == 0 {
		return ""
	}
	return "д. " + strings.Join(parts, ", ")
}

type Subscription struct {
	ID                  uuid.UUID
	CreatedAt           time.Time
	SubscribeToKladr    string
	TgID                string
	SubscribeToFulltext string
}
