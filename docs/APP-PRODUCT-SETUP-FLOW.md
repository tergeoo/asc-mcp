# App Store Connect: правильный флоу регистрации приложения и продуктов

> Пошаговый порядок настройки приложения и его продуктов (подписки + IAP) через
> этот MCP-сервер, **чтобы всё прошло с первого раза**. Порядок важен — половина
> ошибок ниже возникает именно из-за нарушенной последовательности.
>
> Каждый шаг указывает MCP-tool. Идентификаторы (appId, versionId, …) берутся из
> ответов предыдущих шагов (`list_apps`, `get_app_info`, `list_versions`, …).

---

## 0. Предусловия (вне API)

Эти вещи **нельзя сделать API-ключом** — только в веб-консоли App Store Connect:

1. **Запись приложения** (App record). API не умеет создавать приложения
   (`apps: CREATE not allowed`). Создаётся один раз вручную:
   App Store Connect → Apps → ＋ → New App (Platform, Name, Primary Language,
   Bundle ID, SKU).
2. **Paid Applications Agreement** + банк/налоги — обязательны до создания любых
   **платных** продуктов. Подписывает Account Holder. (Бесплатное приложение и
   IAP-*записи* можно создавать и до этого, но цена/доступность платных
   продуктов без активного соглашения не примется.)
3. **ASC API-ключ** (.p8 + Issuer ID + Key ID) с ролью **App Manager**/**Admin**.

После этого `list_apps` должен показать приложение — берём его `appId`.

---

## 1. Метаданные приложения (листинг)

Порядок внутри версии не критичен, но делается до сабмита.

1. `get_app_info` → берём `appInfo.id` и список локалей.
2. **appInfo-локализации** (name, subtitle, privacyPolicyUrl — это уровень
   appInfo, общий для всех версий):
   - первая локаль обычно уже есть (primary language) → `update_app_info_localization`
   - новые локали → `create_app_info_localization`
3. `list_versions` → берём `versionId` версии в состоянии
   **`PREPARE_FOR_SUBMISSION`** (только в нём редактируются тексты).
4. **version-локализации** (description, keywords, whatsNew, promotionalText,
   marketingUrl, supportUrl):
   - `bulk_update_version_localizations` — лучший вариант: апсертит сразу
     несколько локалей (создаёт недостающие, обновляет существующие).
   - либо `update_version_localization` / `create_version_localization`.

### Лимиты полей (валидируются сервером ДО запроса)

| Поле | Лимит |
|---|---|
| name | 30 |
| subtitle | 30 |
| keywords | 100 |
| promotionalText | 170 |
| description / whatsNew | 4000 |
| IAP/subscription: name | 30 |
| IAP/subscription: description | 45 |

### Грабли листинга

- **`whatsNew` нельзя задавать у ПЕРВОЙ версии** (1.0) — Apple вернёт
  `STATE_ERROR: Attribute 'whatsNew' cannot be edited`. Появляется с 1.1+.
- При добавлении новой **appInfo**-локали ASC **сам создаёт** соответствующую
  **version**-локаль (пустую). Поэтому version-локаль той же локали потом надо
  **обновлять**, а не создавать (используй `bulk_…` — он апсертит).
- Тексты редактируются только в `PREPARE_FOR_SUBMISSION` / `DEVELOPER_REJECTED`
  и подобных. В `WAITING_FOR_REVIEW`/`READY_FOR_SALE` — ошибка.

---

## 2. Доступность и цена приложения

1. `set_app_availability` — `allTerritories: true` (по умолчанию) → все 175
   территорий + автоподключение новых.
2. `set_app_price` — `price: "0"` для бесплатного, либо `"4.99"` и т.п.
   (резолвится в app price point базовой территории, остальные выравниваются
   автоматически).

---

## 3. Подписки (auto-renewable)

**Порядок строго такой** — каждый следующий шаг зависит от предыдущего:

1. `create_subscription_group` (appId, referenceName) → `groupId`
2. `set_subscription_group_localization` (groupId, locale, name) — **для каждой
   локали приложения**. Display name группы — обязательная метадата.
3. `create_subscription` (groupId, name, productId, **subscriptionPeriod**:
   `ONE_WEEK`/`ONE_MONTH`/…/`ONE_YEAR`) → `subscriptionId`
4. **`set_subscription_availability` (allTerritories) — ОБЯЗАТЕЛЬНО ДО цены.**
   У подписки без availability нет валидных price points, и `set_subscription_price`
   упадёт с туманным `409 ENTITY_ERROR.RELATIONSHIP.INVALID` на
   `subscriptionPricePoint/id`. Ошибка вводит в заблуждение — реальная причина в
   отсутствии availability.
5. `set_subscription_price` (subscriptionId, price) — ставит цену **во всех
   территориях** (базовая USA + выравнивание через equalizations). Подписочные
   цены, в отличие от приложения/IAP, **НЕ выравниваются автоматически** — если
   задать только одну территорию, подписка останется `MISSING_METADATA`.
6. `update_subscription_localization` (subscriptionId, locale, name, description)
   — для каждой локали.
7. `upload_product_review_screenshot` (productId=subscriptionId,
   productType=`subscription`, filePath) — см. §5 про требования к картинке.

---

## 4. IAP (consumable / non-consumable)

1. `create_iap` (appId, name, productId, type:
   `CONSUMABLE`/`NON_CONSUMABLE`/`NON_RENEWING_SUBSCRIPTION`) → `iapId`
2. **`set_iap_availability` (allTerritories) — ОБЯЗАТЕЛЬНО** (как у подписок;
   без неё IAP застрянет в `MISSING_METADATA`).
3. `set_iap_price` (iapId, price) — для платного; цена выравнивается
   автоматически (price schedule с базовой территорией).
4. `update_iap_localization` (iapId, locale, name, description) — для каждой локали.
5. `upload_product_review_screenshot` (productId=iapId, productType=`iap`, filePath).

---

## 5. Review-скриншоты продуктов (требования к картинке)

Картинка-пейволл, которую увидит ревьюер. **Не требует, чтобы продукты
подгружались в приложении** — подойдёт репрезентативный мокап экрана покупки.

- **Без альфа-канала.** PNG с прозрачностью (RGBA) Apple отвергает (ассет уходит
  в `assetDeliveryState=FAILED`). Самое надёжное — **JPEG** (у него нет альфы).
- **Стандартное разрешение.** Нестандартные размеры (напр. 1320×2868 у новых
  iPhone) могут не приняться. Безопасно: **1242×2688** (6.5") или 1290×2796.
- Один review-скрин на продукт. Чтобы перезалить — старый надо удалить (tool
  делает это сам: **replace-семантика**).
- macOS one-liner для подготовки:
  `sips -z 2688 1242 -s format jpeg in.png --out out.jpg`
- После загрузки ассет обрабатывается асинхронно: успех — `assetDeliveryState=COMPLETE`,
  отказ — `FAILED` (тогда проверь размер/альфу).

---

## 6. Когда продукт становится READY_TO_SUBMIT

`MISSING_METADATA` уходит в `READY_TO_SUBMIT`, только когда собрано **ВСЁ**:

| Подписка | IAP |
|---|---|
| localization (name+description) | localization (name+description) |
| price **во всех территориях** | price (если платный) |
| availability | **availability** |
| review screenshot = COMPLETE | review screenshot = COMPLETE |
| group localization | — |
| duration | — |

Если застряло — почти всегда не хватает **availability** или **цены в части
территорий** (для подписок), либо review-скрин в `FAILED`.

---

## 7. Сабмит (когда всё READY_TO_SUBMIT)

1. Скриншоты **приложения** по устройствам → `upload_screenshot`
   (displayType, напр. `APP_IPHONE_67`).
2. `create_review_submission` (appId, platform) → `submissionId`
3. `add_version_to_submission` (submissionId, versionId)
   (продукты привязываются к сабмиту версии автоматически/отдельными items).
4. `submit_for_review` (submissionId)
5. `get_submission_status` (submissionId) — следить за статусом.

---

## 8. Технические нюансы API (заложены в сервере)

- **inline-id `${...}`**: объекты в массиве `included` (territory availabilities,
  manual prices) должны иметь локальный id формата `${...}` (напр. `${ta-USA}`),
  не голую строку — иначе `id must be a local id with the format '${local-id}'`.
- **Пагинация price points**: у территории бывает **>200** price points — без
  пагинации легко выбрать не ту цену (брался «ближайший» из первой страницы).
  Сервер тянет все страницы (`links.next`).
- **Идемпотентность**: повторный вызов с теми же данными безопасен; цены по
  территориям пропускают уже выставленные (`already exists`).
- **dry-run**: `ASC_DRY_RUN=true` — валидация и diff без записи.

---

## TL;DR порядок

```
App record (web) + Paid Apps Agreement (web)
└─ list_apps → get_app_info
   ├─ appInfo loc (update/create) ; version loc (bulk)
   ├─ set_app_availability(all) ; set_app_price(0|N)
   ├─ Подписки: group → group loc → subscription → AVAILABILITY → price(all) → loc → review shot
   └─ IAP:       create → AVAILABILITY → price → loc → review shot
→ все продукты READY_TO_SUBMIT
→ upload_screenshot (приложение) → create_review_submission → add_version → submit_for_review
```
