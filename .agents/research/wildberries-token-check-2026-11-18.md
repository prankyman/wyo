# Wildberries API token: how a seller makes one, and what is inside it (read 2026-11-18)

Research only. Every fact below is written from Wildberries' own documents, with the address and the date it was read
(2026-11-18). Where a fact could not be found in them it says **NOT SOURCED**. Quotations are in Russian, as Wildberries wrote them.

## 0. Sources and how each was read

| Short name | What it is | Address or file | Read how |
| --- | --- | --- | --- |
| **KB-create** | Knowledge-base article «Как создать, обновить или удалить токен WB API», page text says «Обновлено 18.05.2026» | live: `https://seller.wildberries.ru/instructions/ru/ru/material/how-to-create-update-or-delete-a-wb-api-token` (HTTP 200); saved copy: `docs/marketplaces/WB - Как создать, обновить или удалить токен WB API.pdf` (12 pages) | The live page is a JavaScript shell, but the article text is embedded in the page's own data block; extracted from it. The PDF's text was extracted with a small script (the file reader's PDF mode needs `pdftoppm`, which is not installed). |
| **KB-connect** | Article «Как подключиться по API через токен», «Обновлено 28.05.2026» | saved copy only: `docs/marketplaces/WB - Как подключиться по API через токен.pdf` (6 pages). Live address `https://seller.wildberries.ru/instructions/ru/ru/material/api-integration-with-token` is named inside the descriptions but was **not fetched** (not asked). | PDF text |
| **KB-cloud** | Article «Облачный сервис или локальное решение: как отличить и какой токен выбрать», «Дата обновления: 16.09.2026» | saved copy only: `docs/marketplaces/WB - Облачный сервис или on-premise — какой токен WB API нужен.pdf` (5 pages). Its live address is not printed in the PDF: **NOT SOURCED**. | PDF text |
| **DESC** | Wildberries' own OpenAPI description, section «Авторизация» (`x-description`, line 177) and the `/api/v1/seller-info` method | `api/wildberries/01-general.yaml`, downloaded 2026-11-17 from `https://dev.wildberries.ru/api/swagger/yaml/ru/01-general.yaml` | `grep` and reading in this repository |
| **RFC** | The standard Wildberries points at for `exp` | `https://www.rfc-editor.org/rfc/rfc7519.txt`, HTTP 200, section 4.1.4 and the definition of NumericDate | `curl` |

Notes on access:

- `https://dev.wildberries.ru/docs/openapi/api-information` answers **HTTP 498** to a scripted request and returns an anti-bot
  challenge page («/__wbaas/challenges/antibot/…»), not documentation. So the developer portal's own pages (including its
  «JWT decode» page `/jwt` and `/sandbox`) could not be read live. Their text was read only through DESC, which is the same text
  the portal shows.
- The address given in the request, `https://seller.wildberries.ru/instructions/ru/ru/material/token-wb-api`, answers 200 but the
  page shows «Такой страницы нет. Возможно, страница удалена или в её адресе была ошибка»: no article has that name. The real name
  of the article is `how-to-create-update-or-delete-a-wb-api-token` (article id `A-1468` in the page's menu data).
- Live page against saved PDF (KB-create): compared word by word. **They agree.** Same date line «Обновлено 18.05.2026», same
  rules, same steps, same numbers (180 days, 20 tokens, «Дублировать», 160-170 days). The only differences are layout: the PDF has
  a cookie banner («Мы используем cookies, чтобы»), numbered lists printed with the numbers apart, and a different order of some
  call-out boxes.

## 1. How a seller makes a token, press by press

Rules printed at the top of KB-create («Важные правила»):

- «создавать и удалять токены может только владелец профиля»
- «срок действия токена — 180 дней. За 3 дня до истечения срока владельцу придёт уведомление на портале WB Partners»
- «токен показывается только один раз при создании — сразу сохраните его в надёжном месте»
- «при смене владельца токены перестают работать корректно — новому владельцу нужно будет создать их заново»
- «можно создать до 20 активных токенов на один магазин»

### 1.1 Common first steps (KB-create, «Выберите тип токена»; DESC, «Как создать персональный, базовый или тестовый токен»)

1. «Перейдите в раздел «Интеграции по API» в профиле» (page `https://seller.wildberries.ru/api-integrations`).
2. «нажмите «+ Создать токен». Откроется окно создания токена с двумя вкладками.»
3. Choose the tab by the token type:
   - «Для сервиса из каталога» — «для сервисного токена»
   - «Для интеграции вручную» — «для персонального, базового и тестового токенов»

### 1.2 Personal token («Персональный токен») — the one for a program on the seller's own server (KB-create, «Как создать персональный токен»)

1. Tab «Для интеграции вручную», pick «Персональный токен». A warning appears that the token must not be used with cloud services.
2. Fill in the parameters:
   - Name. «В нём укажите, для подключения какой системы вы создаете этот токен. Например: «ERP-система компании».»
   - Categories: «Отметьте галочками категории данных WB API, к которым нужен доступ.»
   - Access level, «Выберите один из уровней доступа»: «чтение и запись — по токену можно будет получать данные из WB Partners и
     изменять их на портале» or «только чтение — по токену можно будет только получать данные, изменять их не получится».
   - Comment (optional): «Комментарий будет виден всем пользователям вашего профиля на карточке токена. Изменить комментарий
     после создания нельзя.»
   - Tick the box «Я понимаю, что не следует передавать токен третьим лицам».
3. Press «Создать». A window with the token appears. Press «Скопировать и закрыть»: «окно закроется, а токен скопируется в буфер
   обмена.» «Токен показывается один раз.» «Скопировать токен второй раз не получится, но карточку созданного токена с основной
   информацией — срок действия, название системы, уровень доступа, категории данных, — можно в любой момент посмотреть в разделе
   «Интеграции по API».»

### 1.3 Basic token («Базовый токен») (KB-create, «Как создать базовый токен»)

Same as 1.2 with these differences: tab «Для интеграции вручную» → «Базовый токен»; no risk checkbox; «Из соображений безопасности
базовый токен не даёт доступа к некоторым категориям данных — они доступны только через персональный токен».

### 1.4 Service token («Сервисный токен») — only for a cloud service listed in the Catalogue (KB-create, «Как создать сервисный токен»)

Tab «Для сервиса из каталога» → find the service by search → (optional OAuth 2.0 route «Подключить упрощённо по OAuth 2.0») →
categories and name are filled in automatically → «Создать» → «Скопировать и закрыть». «Сервисный токен будет работать только с
выбранным сервисом.» Not relevant for the wyo server, which is not a Catalogue service: **NOT SOURCED** whether a self-installed
program may ever get one.

### 1.5 Test token («Тестовый токен») (KB-create, «Как создать тестовый токен»)

Tab «Для интеграции вручную» → «Тестовый токен» → «Создать». «Категории данных заполнять не нужно — токен автоматически открывает
все категории, доступные в песочнице. Уровень доступа также задан по умолчанию — чтение и запись». «Тестовый токен работает только
с песочницей и не даёт доступа к реальным данным магазина.»

### 1.6 Which type to choose (KB-cloud, «Схема выбора токена»; KB-connect; DESC)

- «Локальная версия (программа на ваших компьютерах или серверах) → Персональный токен»
- «Облачная версия, сервис из Каталога решений → Сервисный токен»
- «Облачная версия, сервис не из Каталога → Базовый токен»
- «Арендованный VPS/VDS сервер. Если вы арендуете виртуальный сервер и сами устанавливаете программы — это локальная версия (у вас
  полный доступ). Нужен Персональный токен.» (KB-cloud, «Особые случаи»)
- «Персональный токен нельзя передавать третьим лицам и использовать с облачными сервисами.» (KB-cloud)

What each type is for, in Wildberries' words (DESC «Правила использования токенов доступа к API», same in KB-connect):

| Type | «Назначение» / limits |
| --- | --- |
| Персональный | «Эксклюзивный токен с расширенными возможностями. Предназначен для того, чтобы предоставлять доступ к данным продавца только вашим собственным программам — в том числе корпоративным системам (on-premise), развёрнутым на собственной или арендованной инфраструктуре.» KB-connect adds: «часть категорий данных доступна только через персональный токен и недоступна в других типах. Например, управление пользователями.» |
| Сервисный | «Специальный токен для подключения конкретного облачного сервиса из официального Каталога готовых решений для бизнеса». «Привязан к конкретному сервису и работает только с ним.» |
| Базовый | «Вспомогательный токен, который предоставляет доступ к ограниченному набору данных продавца и используется во всех случаях, когда не подходит сервисный или персональный токен.» Fit for «тестирования интеграции на реальных данных перед запуском». |
| Тестовый | «Специальный токен для безопасного тестирования и отладки интеграций в изолированной среде — песочнице WB API.» «Реальные данные магазина при этом недоступны.» |

Request limits depend on the type (DESC, `/api/v1/seller-info` table, and the section on limits). For `GET
https://common-api.wildberries.ru/api/v1/seller-info`: Персональный, Сервисный and «Базовый с секретом» — 1 request per minute
(burst 10); **«Базовый» — 1 request per 24 hours (burst 1)**. «Базовый с секретом» is explained only as a basic token «от
зарегистрированных и авторизованных сервисов, которые не включены в Каталог»; what the «секрет» is and how a seller's own program
gets one: **NOT SOURCED**.

### 1.7 Renew / replace (KB-create, «Как обновить токен»)

«Токены действуют 180 дней. После этого они перестают работать, и интеграция прекратит работу, если токен не обновить.» «Продлить
токен технически невозможно. Нужно создать новый токен до истечения срока действия старого.» Steps:

1. «Откройте раздел «Интеграции по API» в профиле.»
2. «Найдите токен, который нужно обновить.»
3. «Нажмите кнопку «Дублировать» — откроется окно создания нового токена с теми же настройками. Подтвердите создание и скопируйте
   новый токен.»
4. «Замените старый токен на новый во всех системах, где он используется.»
5. «Проверьте работу интеграции с новым токеном в течение нескольких дней.»
6. «Удалите старый токен только после того, как убедитесь, что новый работает корректно.»

Advice: «обновлять за 7–14 дней до истечения срока»; reminder in the calendar «за 2 недели»; «примерно через 160-170 дней» (after
creation). Notice: «За 3 дня до окончания срока действия вы получите уведомление на портале WB Partners.»

### 1.8 Delete (KB-create, «Как удалить токен»)

1. «Перейдите в раздел «Интеграции по API».»
2. «Найдите нужный токен в списке и нажмите «Удалить».»
3. «Подтвердите удаление.»
4. «Создайте новый токен и обновите его в настройках системы, с которой работает интеграция.»

A lost token cannot be shown again: «Токен показывается только один раз, при создании. Если вы не сохранили его, придётся удалить
старый токен и создать новый.»

### 1.9 Other rules that matter to the wyo server

- Life: 180 days from creation (KB-create «Важные правила»; KB-connect «Срок действия одного токена — 180 дней»; DESC «Он действует
  180 дней после создания»). Confirmed in three places.
- Owner change: KB-create says tokens «перестают работать корректно». KB-connect says more: «При смене номера владельца личного
  кабинета все токены нужно перевыпустить … начнётся период карантина. В это время интеграции продолжат работать, но часть
  API-запросов будет выполняться с ошибками.» «После завершения карантина все токены, выпущенные до смены номера телефона, будут
  автоматически отозваны». Length of the quarantine: **NOT SOURCED**.
- Header: «Добавляйте токен в заголовок запроса `Authorization`.» (DESC). The method definitions use the security scheme
  `HeaderApiKey`.
- Wrong token answers: 401 «Пользователь не авторизован» («Токен не должен: быть просроченным, быть некорректным, отсутствовать в
  запросе»); 403 «Доступ запрещён» («Категория токена должна совпадать с категорией API»; «Метод должен быть доступен по вашему
  типу токена») (DESC, table of answers, lines 90-92).
- Which category orders need: DESC token-category table, row «Маркетплейс»: «Склады продавца, Остатки на складах продавца, Заказы
  FBS, Заказы DBW, DBS, Самовывоз». Row «Статистика»: «Основные отчёты». Every method in `03-orders-fbs.yaml` carries
  `x-category: marketplace`.
- Read-only: the order-reading methods (`GET /api/v3/orders/new`, `GET /api/v3/orders`, `POST /api/v3/orders/status`, …) carry
  `x-readonly-method: true` in `03-orders-fbs.yaml`; methods that change data (`PATCH /api/v3/orders/{orderId}/cancel`, supplies
  create and delete, …) carry `false`. DESC gives **no sentence explaining the flag**; that a «Только чтение» token is accepted on
  every method flagged `true` is therefore an inference, **NOT SOURCED** as a statement. It must be checked against the fake or
  a real read-only token before wyo's instructions promise it.
- Security advice (KB-create): «Ограничьте доступ к токенам», store in a password manager, one token per integration («создайте для
  них отдельные токены»), «Выбирайте только те категории, с которыми вы планируете работать» (DESC).

## 2. The token's own fields

DESC, section «Как устроен токен»: «Токен представляет собой JWT согласно RFC 7519. Чтобы проверить, действителен ли ваш токен и
какие категории методов API по нему доступны, вы можете декодировать его» (on the developer portal's page `/jwt`).
«Рекомендуем не просматривать токен с помощью внешних онлайн-инструментов, чтобы он не попал в чужие руки». «Остальные поля
`payload` служебные и могут быть удалены.» (So only the fields listed below are promised; others may be absent.)

### 2.1 Verdict on each earlier claim

| Earlier claim | Verdict | Official text |
| --- | --- | --- |
| `exp` = end of life | **Confirmed**, with one correction in wording | DESC table: `exp`, `uint`, «Время жизни токена. Соответствует стандарту RFC 7519 … section-4.1.4». RFC 7519 §4.1.4: «The "exp" (expiration time) claim identifies the expiration time on or after which the JWT MUST NOT be accepted for processing»; its value is a NumericDate: «the number of seconds from 1970-01-01T00:00:00Z UTC». So `exp` is the moment (Unix seconds) the token stops working, not a length of time. Wildberries' word «Время жизни» is loose; the pointer to the RFC settles it. |
| `sid` = the seller profile id, same value as `seller-info` | **Confirmed** | DESC token table: `sid`, `UUIDv4`, «Уникальный ID продавца на Wildberries, которому принадлежит токен». DESC `GET /api/v1/seller-info` (server `https://common-api.wildberries.ru`), answer field `sid`, `format: UUID`: «Уникальный ID продавца на Wildberries, находящийся в публичном поле токена». The method also returns `name`, `tin` (ИНН), `tradeMark`, `country`. Example in DESC: `sid: e8923014-e233-47q8-898e-3cc86d67ea61` (not a valid UUID, an invented sample). The method's wording says «ID его профиля». |
| `acc` = type: 1 Базовый, 2 Тестовый, 3 Персональный, 4 Сервисный | **Confirmed** | DESC table «Значение `acc`»: Базовый `1`; Тестовый `2`; Персональный `3`; Сервисный `4`. |
| `t` = true for a test token | **Confirmed** | Same table, column «Значение `t`»: Тестовый — `true`; the other three — `false`. |
| `s` = bit mask of categories | **Confirmed** | DESC: «Поле `s` — это битовая маска, то есть целое число, каждый бит которого означает наличие или отсутствие какого-то свойства.» Type `uint`. |
| bit 4 «Маркетплейс», bit 5 «Статистика», bit 30 «только на чтение» | **Confirmed** | Same table, below. |

Two fields the earlier research did not list, both in DESC:

- `id` — `UUIDv4`, «Уникальный ID токена».
- `for` — «Поле отсутствует» for Базовый and Тестовый; `self` for Персональный; `asid:{ID сервиса}` for Сервисный.

### 2.2 Bit numbering convention

DESC: «Позиция бита отсчитывается от 0, где 0 — это младший бит.» (Bit 0 is the lowest bit.) So the value of bit N is `1 << N`:
bit 4 = 16, bit 5 = 32, bit 30 = 1 073 741 824. The earlier worry «1<<3?» is refuted by this sentence.

«Значения бит» (every row reads «Свойство (если бит равен 1)»):

| Bit | Value | Meaning in the table |
| --- | --- | --- |
| 1 | 2 | Доступ к категории **Контент** |
| 2 | 4 | Доступ к категории **Аналитика** |
| 3 | 8 | Доступ к категории **Цены и скидки** |
| 4 | 16 | Доступ к категории **Маркетплейс** |
| 5 | 32 | Доступ к категории **Статистика** |
| 6 | 64 | Доступ к категории **Продвижение** |
| 7 | 128 | Доступ к категории **Вопросы и отзывы** |
| 9 | 512 | Доступ к категории **Чат с покупателями** |
| 10 | 1024 | Доступ к категории **Поставки** |
| 11 | 2048 | Доступ к категории **Возвраты покупателями** |
| 12 | 4096 | Доступ к категории **Документы** |
| 13 | 8192 | Доступ к категории **Финансы** |
| 16 | 65536 | Доступ к категории **Пользователи** |
| 30 | 1073741824 | **Токен только на чтение** |

(The value column is computed here from the rule above; the table in DESC lists only positions.) Bits 0, 8, 14, 15 and 17-29 are not
listed: **NOT SOURCED** what they mean (the bit table has no row for them). Test tokens: whether `s` carries all sandbox categories (KB-create: «автоматически открывает все категории, доступные в
песочнице») and whether bit 30 is 0 for them («чтение и запись»): the first part is stated in words, the exact `s` value is **NOT
SOURCED**.

## 3. Sandbox and test token

- «Тестовый токен работает только с песочницей и даёт доступ к сгенерированным тестовым данным. Реальные данные магазина при этом
  недоступны.» (DESC, KB-connect)
- Sandbox servers named in DESC (`servers:`): `https://content-api-sandbox.wildberries.ru` («Песочница Контент»),
  `https://discounts-prices-api-sandbox.wildberries.ru` («Песочница Цены и скидки»), `https://marketplace-api-sandbox.wildberries.ru`
  («Песочница Маркетплейс»), `https://feedbacks-api-sandbox.wildberries.ru` («Песочница Вопросы и отзывы»). The sandbox's own page is
  `https://dev.wildberries.ru/sandbox` (address from DESC; not readable, see section 0).
- Wildberries' advice: «Тестируйте основные сценарии работы в песочнице» (DESC). Whether the sandbox serves the Marketplace orders
  list with data a program can read: **NOT SOURCED** (the sandbox page was not readable).

## 4. Summary of what is NOT SOURCED

1. The live address of KB-cloud (the PDF does not print it).
2. What a «Базовый с секретом» secret is and how a seller's own program obtains one.
3. Whether a «Только чтение» token is accepted on methods flagged `x-readonly-method: true` (the flag is not explained).
4. The length of the quarantine after an owner's phone-number change.
5. The meaning of unlisted bits (0, 8, 14, 15, 17-29) of `s`, and the exact `s` value of a test token.
6. Whether a self-installed program (not in the Catalogue) can obtain a service token.
7. Anything on `dev.wildberries.ru` pages beyond what DESC repeats (the site refuses scripts with HTTP 498).
