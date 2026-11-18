# wyo за другим веб-сервером

Эта страница нужна, только если на вашем сервере уже работает другой сайт и порты 80 и 443 заняты веб-сервером — программой, которая
принимает запросы браузеров (чаще всего это nginx или Caddy). Обычно wyo сам принимает запросы на портах 80 и 443 и сам получает
сертификат для `https://`; два сайта не могут одновременно занимать один порт, поэтому в таком случае wyo ставят «за» ваш веб-сервер.

## Как это работает

1. Установщик запускают с `--behind-proxy`:

    ```
    sudo bash install.sh --behind-proxy
    ```

2. wyo тогда принимает запросы только на адресе `127.0.0.1:8484` — это адрес, доступный лишь программам на самом сервере, а не из
   интернета, — и работает по простому `http`, без сертификата.
3. Ваш веб-сервер принимает запросы браузеров по `https://wyo.example.ru`, держит сертификат и передаёт каждый запрос wyo на
   `127.0.0.1:8484`. Такой веб-сервер называют обратным прокси.

Сертификатом в этом случае занимается ваш веб-сервер, а не wyo, поэтому на странице «Обзор» карточка сертификата говорит, что его
держит веб-сервер перед программой.

Важно: веб-сервер должен передавать заголовок `Host` таким, каким его прислал браузер. wyo по нему отличает запросы своего сайта от
запросов, пришедших с чужого сайта, и отказывает вторым, — так чужая страница не сможет от вашего имени нажать кнопку в wyo.

## Caddy

Caddy сам получает и продлевает сертификат. Добавьте в файл `/etc/caddy/Caddyfile` блок и перезагрузите Caddy
(`sudo systemctl reload caddy`):

```
wyo.example.ru {
    reverse_proxy 127.0.0.1:8484
}
```

Caddy по умолчанию передаёт заголовок `Host` без изменений, поэтому больше ничего не нужно.

## nginx

nginx сам сертификат не получает; обычно для этого используют программу certbot. Пример файла `/etc/nginx/sites-available/wyo`
(сертификат уже получен certbot и лежит в `/etc/letsencrypt/live/wyo.example.ru/`):

```
server {
    listen 80;
    server_name wyo.example.ru;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    http2 on;
    server_name wyo.example.ru;

    ssl_certificate     /etc/letsencrypt/live/wyo.example.ru/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/wyo.example.ru/privkey.pem;

    client_max_body_size 16m;

    location / {
        proxy_pass http://127.0.0.1:8484;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_read_timeout 120s;
    }
}
```

Затем включите сайт и перезагрузите nginx:

```
sudo ln -s /etc/nginx/sites-available/wyo /etc/nginx/sites-enabled/wyo
sudo nginx -t
sudo systemctl reload nginx
```

`nginx -t` проверяет настройки до перезагрузки: если в файле ошибка, он скажет, в какой строке, и работающий сайт не остановится.

## Проверка

На сервере:

```
curl -s http://127.0.0.1:8484/healthz
```

Ответ `ok` значит, что wyo работает. Затем откройте `https://wyo.example.ru` в браузере: должна появиться страница входа. Если
появляется ошибка 502, веб-сервер не может достучаться до wyo: проверьте `sudo systemctl status wyo` и `sudo wyo doctor`.
