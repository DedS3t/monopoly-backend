create table users
(
    id       text not null
        constraint users_pkey
            primary key,
    email    text not null
        constraint unique_email
            unique,
    password text not null
);

alter table users
    owner to postgres;

create table games
(
    id     text not null
        constraint games_pkey
            primary key,
    name   text not null,
    status text,
    type   text
);

alter table games
    owner to postgres;

create table players
(
    id       bigserial not null
        constraint players_pkey
            primary key,
    user_id  text      not null
        constraint unique_user
            unique
        constraint players_user_id_fkey
            references users,
    game_id  text      not null
        constraint players_game_id_fkey
            references games,
    username text      not null,
    active text        not null,
);

alter table players
    owner to postgres;

