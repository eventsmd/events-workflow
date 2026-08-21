// Package workflows — Temporal workflow and activities. Activity names
// (SaveRawMessage, etc.) match the Java version: Temporal extracts the activity
// type from the method name in both implementations.
package workflows

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"

	"events-workflow/internal/ai"
	"events-workflow/internal/domain"
	"events-workflow/internal/events"
	"events-workflow/internal/geo"
	"events-workflow/internal/kladr"
	"events-workflow/internal/messaging"
	"events-workflow/internal/store"
)

const TaskQueue = "TelegramMessageQueue"

type Activities struct {
	Store     *store.Store
	Parser    *ai.MessageParser
	Adapter   *geo.Adapter
	Sender    *messaging.SQSSender
	Publisher *events.Publisher
}

// SaveRawMessage — port: incident_id is inherited from the reply-to message,
// otherwise (or if reply-to is absent) a new one is generated.
func (a *Activities) SaveRawMessage(ctx context.Context, msg domain.TelegramMessage) error {
	incidentID := uuid.New()
	if msg.ReplyTo != nil {
		parent, err := a.Store.FindMessage(ctx, msg.ReplyTo.ID, msg.ReplyTo.ChatID)
		if err != nil {
			return err
		}
		if parent != nil && parent.IncidentID != nil {
			incidentID = *parent.IncidentID
		}
	}
	return a.Store.SaveMessage(ctx, store.TelegramMessageEntityFrom(msg, incidentID))
}

func (a *Activities) ParseMessage(ctx context.Context, msg domain.TelegramMessage) (domain.ParsedMessage, error) {
	tr, err := a.Parser.Parse(ctx, msg.Date, msg.Text)
	if err != nil {
		return domain.ParsedMessage{}, err
	}
	return domain.ParsedMessage{OriginalMessage: msg, MessageTranscription: tr}, nil
}

func (a *Activities) SaveParsedMessage(ctx context.Context, pm domain.ParsedMessage) error {
	if err := a.Store.SaveTranscribe(ctx, store.TranscribeEntityFrom(pm)); err != nil {
		return err
	}
	tr := pm.MessageTranscription
	if tr == nil || len(tr.Addresses) == 0 {
		return nil
	}
	ref := domain.MessageReference{ID: pm.OriginalMessage.ID, ChatID: pm.OriginalMessage.ChatID}
	entities := make([]*store.AddressEntity, len(tr.Addresses))
	for i, addr := range tr.Addresses {
		entities[i] = store.AddressEntityFrom(addr, ref)
	}
	return a.Store.SaveAddresses(ctx, entities)
}

// NormalizeAddress — errors during enrichment of individual addresses are logged
// and swallowed (like try/catch in the Java version): one bad address must not
// fail processing of the rest.
func (a *Activities) NormalizeAddress(ctx context.Context, pm domain.ParsedMessage) error {
	if pm.MessageTranscription == nil {
		return nil
	}
	for _, addr := range pm.MessageTranscription.Addresses {
		entity, err := a.Store.FindAddress(ctx, addr.ID)
		if err != nil {
			return err
		}
		if entity == nil {
			continue
		}
		if err := a.enrichAndSave(ctx, pm, entity); err != nil {
			slog.Error("Error on processing address",
				"city", strDeref(entity.CityName), "street", strDeref(entity.StreetName),
				"error", err)
		}
	}
	return nil
}

func (a *Activities) enrichAndSave(ctx context.Context, pm domain.ParsedMessage, entity *store.AddressEntity) error {
	if err := a.Adapter.Enrich(ctx, pm, entity); err != nil {
		return err
	}
	return a.Store.SaveAddress(ctx, entity)
}

func (a *Activities) Notify(ctx context.Context, id, chatID int64) error {
	addrs, err := a.Store.FindAddressesByMessage(ctx, id, chatID)
	if err != nil {
		return err
	}
	transcribe, err := a.Store.FindTranscribe(ctx, id, chatID)
	if err != nil {
		return err
	}
	message, err := a.Store.FindMessage(ctx, id, chatID)
	if err != nil {
		return err
	}
	if transcribe == nil || message == nil {
		return nil // ifPresent in Java: silently skip
	}
	for _, addr := range addrs {
		kladrRaw := firstNonNil(addr.StreetKladr, addr.CityKladr, addr.RegionKladr)
		if kladrRaw == nil {
			continue
		}
		code, err := kladr.Parse(*kladrRaw)
		if err != nil {
			return err // Java throws IllegalArgumentException → retry activity
		}
		subs, err := a.Store.FindSubscriptionsByKladrPrefix(ctx, code.Prefix())
		if err != nil {
			return err
		}
		for _, sub := range subs {
			// Java only accesses message.getContext().get("supplier") and
			// messageTranscribe.getEventStart() inside this per-subscription
			// lambda, so a zero-subscriber address never NPEs. Mirror both:
			// checked here (not before the subs loop), and — per the
			// controller's fail-loud ruling — converted to an explicit error
			// rather than silently sending a degraded notification.
			if message.Context == nil {
				return fmt.Errorf("message %d:%d has no context (supplier unknown)", id, chatID)
			}
			supplier := message.Context["supplier"]
			// Restoration messages ("... восстановлено") routinely name no
			// time, so the model leaves event_start empty. For a resume the
			// time is implied — restored as of the message itself — so the
			// message's own date stands in (its value, not time.Now(), so a
			// workflow reset days later still reports when the supply
			// actually came back). Deliberately confined to resume and to
			// the notification text only: the DB and the published event
			// keep the empty event_start, and for shutdown/other the start
			// time is the whole point, so those still fail loudly.
			eventStart := transcribe.EventStart
			if eventStart == nil && strDeref(transcribe.Event) == "resume" {
				eventStart = &message.Date
			}
			if eventStart == nil {
				return fmt.Errorf("event_start is nil for %d:%d", id, chatID)
			}
			houses := addr.FormatHouses()
			addressText := sub.SubscribeToFulltext
			if houses != "" {
				addressText = fmt.Sprintf("%s, %s", sub.SubscribeToFulltext, houses)
			}
			text := NotificationText(supplier, strDeref(transcribe.Event),
				addressText, *eventStart, strDeref(transcribe.Description))
			tgID, err := strconv.ParseInt(sub.TgID, 10, 64)
			if err != nil {
				return fmt.Errorf("bad tg_id %q: %w", sub.TgID, err)
			}
			if err := a.Sender.Send(ctx, messaging.MessageToSend{
				TelegramID: tgID, Message: text, Topic: supplier,
				MessageID: id, ChatID: chatID,
			}); err != nil {
				return err
			}
			slog.Info("Notify client", "tgId", sub.TgID, "address", sub.SubscribeToFulltext)
		}
	}
	return nil
}

// NotificationText — message format from MessageProcessActivity.notify.
func NotificationText(supplier, event, addressText string, start time.Time, description string) string {
	var emoji, serviceName string
	switch supplier {
	case "water":
		emoji, serviceName = "💧", "водоснабжения"
	case "electricity":
		emoji, serviceName = "⚡️", "электроснабжения"
	}
	var eventDescription string
	switch event {
	case "shutdown":
		eventDescription = "Отключение"
	case "resume":
		eventDescription = "Возобновление"
	}
	return fmt.Sprintf("%s %s услуги %s по адресу «%s» с %s\n\n%s",
		emoji, eventDescription, serviceName, addressText,
		start.Format("02-01-2006 15:04"), description)
}

func (a *Activities) PublishEvent(ctx context.Context, id, chatID int64) error {
	message, err := a.Store.FindMessage(ctx, id, chatID)
	if err != nil {
		return err
	}
	transcribe, err := a.Store.FindTranscribe(ctx, id, chatID)
	if err != nil {
		return err
	}
	if message == nil || transcribe == nil {
		slog.Info("Skip publishEvent — message or transcript missing", "id", id, "chatId", chatID)
		return nil
	}
	var supplier string
	if message.Context != nil {
		supplier = message.Context["supplier"]
	}
	addrs, err := a.Store.FindAddressesByMessage(ctx, id, chatID)
	if err != nil {
		return err
	}
	eventAddresses := make([]events.EventAddress, 0, len(addrs))
	for _, addr := range addrs {
		eventAddresses = append(eventAddresses, events.EventAddress{
			Region: kladrRef(addr.RegionName, addr.RegionKladr, addr.RegionType),
			City:   kladrRef(addr.CityName, addr.CityKladr, addr.CityType),
			Street: kladrRef(addr.StreetName, addr.StreetKladr, addr.StreetType),
			Houses: addr.Houses(),
		})
	}
	var incidentID string
	if message.IncidentID != nil {
		incidentID = message.IncidentID.String()
	}
	ev := events.UtilityEvent{
		IncidentID:   incidentID,
		Supplier:     supplier,
		Event:        strDeref(transcribe.Event),
		Organization: strDeref(transcribe.Organization),
		Description:  strDeref(transcribe.Description),
		EventStart:   minuteOf(transcribe.EventStart),
		EventStop:    minuteOf(transcribe.EventStop),
		PublishedAt:  time.Now().UTC(),
		Source:       &events.EventSource{MessageID: id, ChatID: chatID},
		Addresses:    eventAddresses,
	}
	a.Publisher.Publish(ctx, ev) // errors logged inside — best-effort, like in Java
	return nil
}

func minuteOf(t *time.Time) *domain.MinuteDateTime {
	if t == nil {
		return nil
	}
	return &domain.MinuteDateTime{Time: *t}
}

func kladrRef(name, code, typ *string) *events.KladrRef {
	if name == nil && code == nil && typ == nil {
		return nil
	}
	return &events.KladrRef{Name: strDeref(name), Kladr: strDeref(code), Type: strDeref(typ)}
}

func firstNonNil(ptrs ...*string) *string {
	for _, p := range ptrs {
		if p != nil {
			return p
		}
	}
	return nil
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
