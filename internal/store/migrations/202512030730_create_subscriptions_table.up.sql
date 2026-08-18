create table if not exists subscriptions
(
    id                    uuid      not null
        constraint subscriptions_pk
            primary key,
    created_at            timestamp not null,
    subscribe_to_kladr    text      not null,
    tg_id                 text      not null,
    subscribe_to_fulltext text      not null,
    constraint subscriptions_uq
        unique (subscribe_to_kladr, tg_id)
);

create index if not exists subscriptions_subscribe_to_kladr_index
    on subscriptions (subscribe_to_kladr);