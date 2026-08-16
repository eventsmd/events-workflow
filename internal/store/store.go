package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool — pgxpool с регистрацией типа hstore (нестандартный OID,
// pgx требует явной загрузки).
//
// Адаптация от брифа: pgx v5.10.0's Conn.LoadType трактует любой
// catalog-типтайп 'b' как массив (смотрит typelem), поэтому LoadType
// для самого hstore (скалярный extension-тип, typelem=0) падает с
// "array element OID not registered". Регистрируем кодеки напрямую
// через pgtype.HstoreCodec, вычисляя OID/typarray из pg_type сами.
func NewPool(ctx context.Context, pgURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(pgURL)
	if err != nil {
		return nil, err
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		var hstoreOID, hstoreArrayOID uint32
		err := conn.QueryRow(ctx,
			`SELECT oid, typarray FROM pg_type WHERE typname = 'hstore'`).
			Scan(&hstoreOID, &hstoreArrayOID)
		if err != nil {
			return fmt.Errorf("lookup hstore oid: %w", err)
		}
		hstoreType := &pgtype.Type{Name: "hstore", OID: hstoreOID, Codec: pgtype.HstoreCodec{}}
		conn.TypeMap().RegisterType(hstoreType)
		conn.TypeMap().RegisterType(&pgtype.Type{
			Name: "_hstore", OID: hstoreArrayOID,
			Codec: &pgtype.ArrayCodec{ElementType: hstoreType},
		})
		return nil
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

// NewPoolNoHstore — для миграций/тестов до создания extension hstore.
func NewPoolNoHstore(ctx context.Context, pgURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, pgURL)
}

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool} }

func toHstore(m map[string]string) pgtype.Hstore {
	if m == nil {
		return nil
	}
	h := pgtype.Hstore{}
	for k, v := range m {
		v := v
		h[k] = &v
	}
	return h
}

func fromHstore(h pgtype.Hstore) map[string]string {
	if h == nil {
		return nil
	}
	m := make(map[string]string, len(h))
	for k, v := range h {
		if v != nil {
			m[k] = *v
		}
	}
	return m
}

const msgColumns = `id, chat_id, text, date, service_name, from_id, from_name,
	reply_to_id, reply_to_chat_id, context, incident_id`

func (s *Store) SaveMessage(ctx context.Context, e *TelegramMessageEntity) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO telegram_messages (`+msgColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id, chat_id) DO UPDATE SET
			text = EXCLUDED.text, date = EXCLUDED.date,
			service_name = EXCLUDED.service_name,
			from_id = EXCLUDED.from_id, from_name = EXCLUDED.from_name,
			reply_to_id = EXCLUDED.reply_to_id,
			reply_to_chat_id = EXCLUDED.reply_to_chat_id,
			context = EXCLUDED.context, incident_id = EXCLUDED.incident_id`,
		e.ID, e.ChatID, e.Text, e.Date, e.ServiceName, e.FromID, e.FromName,
		e.ReplyToID, e.ReplyToChatID, toHstore(e.Context), e.IncidentID)
	return err
}

func (s *Store) FindMessage(ctx context.Context, id, chatID int64) (*TelegramMessageEntity, error) {
	var e TelegramMessageEntity
	var h pgtype.Hstore
	err := s.pool.QueryRow(ctx,
		`SELECT `+msgColumns+` FROM telegram_messages WHERE id=$1 AND chat_id=$2`,
		id, chatID).Scan(
		&e.ID, &e.ChatID, &e.Text, &e.Date, &e.ServiceName, &e.FromID, &e.FromName,
		&e.ReplyToID, &e.ReplyToChatID, &h, &e.IncidentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	e.Context = fromHstore(h)
	return &e, nil
}

func (s *Store) SaveTranscribe(ctx context.Context, e *TranscribeEntity) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO telegram_message_transcribes
			(id, chat_id, organization, description, event, event_start, event_stop)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id, chat_id) DO UPDATE SET
			organization = EXCLUDED.organization, description = EXCLUDED.description,
			event = EXCLUDED.event, event_start = EXCLUDED.event_start,
			event_stop = EXCLUDED.event_stop`,
		e.ID, e.ChatID, e.Organization, e.Description, e.Event, e.EventStart, e.EventStop)
	return err
}

func (s *Store) FindTranscribe(ctx context.Context, id, chatID int64) (*TranscribeEntity, error) {
	var e TranscribeEntity
	err := s.pool.QueryRow(ctx, `
		SELECT id, chat_id, organization, description, event, event_start, event_stop
		FROM telegram_message_transcribes WHERE id=$1 AND chat_id=$2`,
		id, chatID).Scan(&e.ID, &e.ChatID, &e.Organization, &e.Description,
		&e.Event, &e.EventStart, &e.EventStop)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

const addrColumns = `id, message_id, chat_id, city_original, street_original,
	street_type_original, house_numbers, house_ranges, region_name, region_kladr,
	region_type, city_name, city_kladr, city_type, street_name, street_kladr, street_type`

const addrUpsert = `
	INSERT INTO incident_address (` + addrColumns + `)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	ON CONFLICT (id) DO UPDATE SET
		message_id = EXCLUDED.message_id, chat_id = EXCLUDED.chat_id,
		city_original = EXCLUDED.city_original, street_original = EXCLUDED.street_original,
		street_type_original = EXCLUDED.street_type_original,
		house_numbers = EXCLUDED.house_numbers, house_ranges = EXCLUDED.house_ranges,
		region_name = EXCLUDED.region_name, region_kladr = EXCLUDED.region_kladr,
		region_type = EXCLUDED.region_type, city_name = EXCLUDED.city_name,
		city_kladr = EXCLUDED.city_kladr, city_type = EXCLUDED.city_type,
		street_name = EXCLUDED.street_name, street_kladr = EXCLUDED.street_kladr,
		street_type = EXCLUDED.street_type`

func addrArgs(e *AddressEntity) []any {
	return []any{e.ID, e.MessageID, e.ChatID, e.CityOriginal, e.StreetOriginal,
		e.StreetTypeOriginal, e.HouseNumbers, e.HouseRanges, e.RegionName,
		e.RegionKladr, e.RegionType, e.CityName, e.CityKladr, e.CityType,
		e.StreetName, e.StreetKladr, e.StreetType}
}

func (s *Store) SaveAddress(ctx context.Context, e *AddressEntity) error {
	_, err := s.pool.Exec(ctx, addrUpsert, addrArgs(e)...)
	return err
}

func (s *Store) SaveAddresses(ctx context.Context, es []*AddressEntity) error {
	for _, e := range es {
		if err := s.SaveAddress(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

func scanAddress(row pgx.Row) (*AddressEntity, error) {
	var e AddressEntity
	err := row.Scan(&e.ID, &e.MessageID, &e.ChatID, &e.CityOriginal,
		&e.StreetOriginal, &e.StreetTypeOriginal, &e.HouseNumbers, &e.HouseRanges,
		&e.RegionName, &e.RegionKladr, &e.RegionType, &e.CityName, &e.CityKladr,
		&e.CityType, &e.StreetName, &e.StreetKladr, &e.StreetType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *Store) FindAddress(ctx context.Context, id uuid.UUID) (*AddressEntity, error) {
	return scanAddress(s.pool.QueryRow(ctx,
		`SELECT `+addrColumns+` FROM incident_address WHERE id=$1`, id))
}

func (s *Store) FindAddressesByMessage(ctx context.Context, messageID, chatID int64) ([]*AddressEntity, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+addrColumns+` FROM incident_address WHERE message_id=$1 AND chat_id=$2`,
		messageID, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AddressEntity
	for rows.Next() {
		e, err := scanAddress(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) FindSubscriptionsByKladrPrefix(ctx context.Context, prefix string) ([]*Subscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, created_at, subscribe_to_kladr, tg_id, subscribe_to_fulltext
		FROM subscriptions WHERE subscribe_to_kladr LIKE $1 || '%'`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.CreatedAt, &s.SubscribeToKladr, &s.TgID,
			&s.SubscribeToFulltext); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

func (s *Store) SaveSubscription(ctx context.Context, sub *Subscription) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO subscriptions (id, created_at, subscribe_to_kladr, tg_id, subscribe_to_fulltext)
		VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO NOTHING`,
		sub.ID, sub.CreatedAt, sub.SubscribeToKladr, sub.TgID, sub.SubscribeToFulltext)
	return err
}
