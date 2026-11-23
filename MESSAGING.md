# Messaging channels

Telegram delivers the daily digest. Set `TELEGRAM_BOT_USERNAME`,
`TELEGRAM_BOT_TOKEN` and `TELEGRAM_WEBHOOK_SECRET` and register
`POST /v1/hunterjob/webhooks/telegram` with the same webhook secret. The
API and digest worker need the Telegram settings.

## Messenger

Set `MESSENGER_PAGE_ID`, `MESSENGER_PAGE_TOKEN`, `MESSENGER_APP_SECRET`
and `MESSENGER_VERIFY_TOKEN` on the API. Configure the Page webhook at
`/v1/hunterjob/webhooks/messenger` with the same verification token and
subscribe to the `messages` field. The Page token needs the Messenger
permissions for the intended users.

From the dashboard, open the Page and send the one-time link command.
Send `/jobs` in that chat to receive the latest matches on demand. Messages
are replies to incoming user requests in the standard messaging window.
[Meta Messenger Platform](https://developers.facebook.com/docs/messenger-platform/)

## Zalo

Set `ZALO_OA_ID`, `ZALO_ACCESS_TOKEN` and `ZALO_WEBHOOK_SECRET` on the API.
Register `POST /v1/hunterjob/webhooks/zalo` for the `user_send_text` event
and supply the OA webhook secret. Keep the OA access token current via your
Zalo credential rotation process.

Open the OA from the dashboard and send the one-time `#link` command.
Send `#jobs` to request the latest matches as an OA consultation reply.
[Zalo OA consultation messaging](https://oa.zalo.me/home/documents/guides/tin-tu-van)
