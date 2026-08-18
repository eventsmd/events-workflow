CREATE EXTENSION IF NOT EXISTS hstore;

CREATE TABLE IF NOT EXISTS telegram_messages (
    id                  bigint not null,
    chat_id             bigint not null,
    text                text not null,
    date                timestamp not null,
    service_name        text,
    -- decomposed `from`
    from_id             bigint not null,
    from_name           text,
    -- decomposed `replyTo`
    reply_to_id         bigint,
    reply_to_chat_id    bigint,
    -- hstore context
    context             hstore,
    incident_id         UUID,
    CONSTRAINT pk_telegram_messages PRIMARY KEY (id, chat_id)
);

-- Helpful indexes
CREATE INDEX IF NOT EXISTS idx_telegram_messages_chat_id ON telegram_messages(chat_id);
CREATE INDEX IF NOT EXISTS idx_telegram_messages_service_name ON telegram_messages(service_name);
CREATE INDEX IF NOT EXISTS idx_telegram_messages_reply_to_id ON telegram_messages(reply_to_id);
CREATE INDEX IF NOT EXISTS idx_telegram_messages_context_gin ON telegram_messages USING GIN (context);

CREATE TABLE incident_address
(
    id                   UUID NOT NULL,
    message_id           bigint NOT NULL,
    chat_id              bigint NOT NULL,
    city_original        text,
    street_original      text,
    street_type_original text,
    house_numbers        text,
    house_ranges         text,
    region_name          text,
    region_kladr         text,
    region_type          text,
    city_name            text,
    city_kladr           text,
    city_type            text,
    street_name          text,
    street_kladr         text,
    street_type          text,
    street_type_raw      text,
    CONSTRAINT pk_addressentity PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_telegram_messages_chat ON incident_address(message_id, chat_id);

CREATE TABLE telegram_message_transcribes
(
    id           bigint NOT NULL,
    chat_id      bigint NOT NULL,
    organization text,
    description  text,
    event        text,
    event_start  TIMESTAMP WITHOUT TIME ZONE,
    event_stop   TIMESTAMP WITHOUT TIME ZONE,
    CONSTRAINT pk_telegram_message_transcribes PRIMARY KEY (id, chat_id)
);
