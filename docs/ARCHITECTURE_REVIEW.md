# Архитектурное ревью: доменная модель

Дата: 2026-04-23
Ветка: main @ b25ef10
Объём кодовой базы: 236 Go-файлов, ~11.4K строк в `pkg/` (без тестов), 17 пакетов.

Фокус ревью: не расползается ли доменная модель, нет ли дублирования одной
концепции в разных пакетах, нет ли избыточного переусложнения. Все находки
перепроверены вручную, где агенты сообщали спорные утверждения — это отмечено.

---

## 1. Резюме

Общее состояние — здоровое для solo-проекта такого объёма. Ядро доменной
модели компактное (`scenario.Scenario`, `environment.Lease`, `verifier.Check`)
и не размазано. Проблемы локализованы на границах — там, где «один и тот же
результат» проходит через 3 sink'а (диск, SQLite, Evidra API), и на стыке
sequential/parallel-режимов исполнения.

Главные болевые точки по убыванию приоритета:

| № | Проблема | Приоритет | Стоимость фикса |
|---|----------|-----------|-----------------|
| 1 | `tui.RunRecord` дублирует `artifact.RunBundle` и вручную парсит `run.json` | высокий | ~1 час |
| 2 | Коллизия имени `ToolCall` в `adapter` и `agent` (разные структуры, одинаковое имя) | средний | ~30 мин (rename) |
| 3 | SQLite теряет `tool_calls`, `transcript`, `prompt`, `chaos` — неполный аудит | высокий | ~4 часа |
| 4 | `harness/run.go` — 1258 строк, линейно соединяет 6 фаз | средний | 1–2 дня |
| 5 | Три `RunResult` в разных пакетах с одинаковым именем — когнитивный шум | средний | ~30 мин (rename) |
| 6 | `workspace.RewriteNamespace` — текстовая подмена `"bench"` в YAML, хрупко | средний | 1 день |
| 7 | Три клиента к Anthropic API (`anthropic.go`, `bifrost.go`, `claude.go`) с независимыми retry/backoff | средний | 0.5 дня |
| 8 | Версии Evidra лежат в `metadata_json` — неиндексируемы для запросов | средний | 2 часа + миграция |
| 9 | `CLAUDE.md` отстал от `ARCHITECTURE.md` — нет упоминаний orchestrator/workspace/jobqueue | низкий | 30 мин |
| 10 | `ProxyEvidenceWriter` — интерфейс на одну реализацию | низкий | 15 мин |

Чего в проекте **нет** и чему можно радоваться:
- Доменное ядро (`Scenario`, `Lease`, `Check`) не дублируется.
- Профили (`default`/`argocd`/`aws-localstack`) централизованы в `pkg/scenario/profile.go` и не растаскиваются по local-флагам.
- `pkg/report` и `pkg/store` физически разделены по назначению (исходящий трафик vs персистентность), а не случайно.
- Нет самописного логгера/HTTP-клиента/CLI-парсера — стоят cobra + stdlib.

---

## 2. Карта ключевых доменных сущностей

| Сущность | Источник истины | Оценка |
|----------|-----------------|--------|
| `Scenario` | `pkg/scenario/types.go:11` | Единая истина. UI-копия в `ui/src/data/catalog.ts` — нормальный cross-language дубль. |
| `ExecutionProfile` | `pkg/scenario/profile.go:5` | Единая истина. |
| `Lease` | `pkg/environment/lease.go:12` | Единая истина, владеется провижинером. |
| `Verifier.Check` / `VerifyResult` | `pkg/verifier/types.go` | Единая истина. |
| `Provider` (LLM) | `pkg/agent/provider.go:9` (interface) | Единая истина для рантайма. `config.Provider` — строковое имя, разные слои, не дубль. |
| `RunResult` | **3 определения** | См. §4.1 — разные контексты, но одно имя создаёт шум. |
| `RunRecord` | **2 определения** + `RunBundle` | См. §3.1 — `tui.RunRecord` — реальный дубль. |
| `ToolCall` | **2 определения** | См. §3.2 — реальный дубль, структуры идентичны. |
| `Job` / `BenchJobArgs` | `pkg/jobqueue/types.go` | Единая истина для параллельного режима. |

Границы доменов видны чётко: сценарий → среда → агент → проверка → артефакт.
Нарушения — только на стыках (sink'и и adapter-vs-agent).

---

## 3. Подтверждённое реальное дублирование

### 3.1 `tui.RunRecord` дублирует `artifact.RunBundle` — высокий

Файлы:
- `pkg/tui/history.go:14` — `type RunRecord struct{...}`
- `pkg/artifact/writer.go:13` — `type RunBundle struct{...}`
- `pkg/store/store.go:18` — `type RunRecord = bench.RunRecord` (алиас на `samebits.com/evidra`)

Что происходит. TUI читает `runs/<run-id>/run.json` с диска и вручную парсит
в свою локальную структуру `RunRecord`, которая описывает ровно те же поля,
что пишет `artifact.RunBundle`. Получается «свой парсер своего же формата»
в соседнем пакете.

Последствия. Любое изменение схемы `run.json` требует синхронного изменения
в двух местах. Сейчас эти места уже расходятся по набору полей (`tui` читает
только часть).

Фикс. Удалить `tui.RunRecord`, читать `artifact.RunBundle` напрямую
(`json.Unmarshal` в `artifact.RunBundle`). Экспорт метода `LoadBundle(dir string)`
из `pkg/artifact/`. Удалить `LoadHistory` в его текущем виде, заменить
на обход runs-dir + `LoadBundle`.

### 3.2 Коллизия имени `ToolCall` — средний (не дубликат, а confusion)

Файлы и реальные структуры:

```go
// pkg/adapter/types.go:34 — ЗАПИСЬ исполненного tool-call'а (для артефакта)
type ToolCall struct {
    Tool      string         `json:"tool"`
    Args      map[string]any `json:"args,omitempty"`
    Result    string         `json:"result,omitempty"`
    Timestamp time.Time      `json:"timestamp"`
}

// pkg/agent/provider.go:41 — ЗАПРОС tool-call'а от LLM (протокол Anthropic/OpenAI)
type ToolCall struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Arguments string `json:"arguments"`
}
```

**Это не дубликат.** Это две разных сущности с одинаковым именем:
- `agent.ToolCall` — что модель **попросила** вызвать (используется в bifrost/anthropic/claude-провайдерах для парсинга ответа).
- `adapter.ToolCall` — что было **вызвано и записано** в историю (сохраняется в `artifact.RunBundle.ToolCalls`).

В `pkg/harness/run.go:1101` уже есть явный конвертер
`providerToolCalls(messages []agent.Message) []adapter.ToolCall` — это
граница между «протоколом LLM» и «записью в артефакт».

Проблема только в имени — одинаковое имя для разных концепций путает.

Фикс (предпочтительный). Переименовать историческую запись:
`adapter.ToolCall` → `adapter.ToolCallRecord` (или перенести в `pkg/artifact/`
как `artifact.ToolCallRecord`, где она фактически и используется).
`agent.ToolCall` оставить как протокольный тип.

Стоимость. 30 минут rename + прогон `go build ./...`. Импортный цикл
между `adapter` и `agent` отсутствует — проверено.

**Исправление предыдущей версии этого документа.** В первой редакции §3.2
ошибочно утверждалось, что структуры идентичны бит-в-бит — это было
неверно, основывалось на сводке одного из агентов без ручной проверки
полей. Фактические поля разные, обе структуры нужны.

### 3.3 SQLite теряет часть полезной нагрузки — высокий

Файлы:
- `pkg/harness/run.go:459` — пишет `artifact.RunBundle` с полным payload.
- `pkg/harness/run.go:519` — в `store.RunRecord.MetadataJSON` идёт только metadata.
- `pkg/harness/run.go:548` — `ReportToEvidra` получает rec + transcript + toolCalls как отдельные аргументы.
- `pkg/store/store.go:18` — `RunRecord` (алиас на `bench.RunRecord` из соседнего репо).

Что теряется в SQLite:

| Данные | artifact/run.json | SQLite | Evidra API |
|--------|------|--------|------------|
| tool_calls | да | **нет** | да (отдельный payload) |
| transcript | да | **нет** | да (отдельный payload) |
| prompt.txt | да | **нет** | **нет** |
| chaos.json/chaos.log | да | **нет** | **нет** |
| checks | да | да (`checks_json`) | да |
| metadata (версии) | да | да (`metadata_json`) | да |

Последствия. Нельзя полностью восстановить run из локальной БД — нужен доступ
к директории артефактов. Для воспроизводимости бенчмарка это ограничение.

Фикс (минимальный).
- Добавить `tool_calls_json`, `transcript`, `prompt` колонки в `bench.RunRecord`
  (совместно с командой evidra, т.к. тип из их репо).
- Для массивных транскриптов — сохранять указатель `artifacts_dir` и читать
  по требованию (уже есть, но не используется при анализе).

### 3.4 Версии Evidra в `metadata_json` — средний

Файл: `pkg/config/versions.go` собирает `VersionInfo` корректно, вызывается
один раз в `pkg/harness/run.go:1025`. Затем сливается в `agentResult.Metadata`,
далее — в `MetadataJSON` (строка). По версиям нельзя построить индекс/фильтр
в SQLite.

Фикс. Выделить в `bench.RunRecord` отдельные колонки
`contract_version`, `skill_version`, `spec_version`, `scoring_version`,
`evidra_version`. Индексировать `contract_version`, `skill_version` — это
основные фильтры для skill-delta-аналитики.

---

## 4. Мнимые дубликаты (оставить как есть)

### 4.1 Три `RunResult` — это 3 bounded context'а, не дубликат

Файлы:
- `pkg/adapter/types.go:24` — выход процесса агента (exit code, stdout, transcript).
- `pkg/harness/run.go:72` — результат сценария (passed, checks, artifacts).
- `pkg/orchestrator/orchestrator.go:122` — агрегат по batch'у (total/passed/failed).

Разные уровни абстракции: сырой процесс → сценарий → пакет сценариев.
Проблема только в одинаковом имени — когнитивный шум.

Фикс (косметический). Переименовать для ясности:
- `adapter.RunResult` → `adapter.AgentOutput`
- `harness.RunResult` → `harness.ScenarioResult`
- `orchestrator.RunResult` → `orchestrator.BatchStats`

Это 30-минутный `gopls rename`. Не обязательно, но упрощает чтение стектрейсов.

### 4.2 `artifact.RunBundle` vs `skilldelta.RunSnapshot` — не дубль

`RunBundle` — полный payload одиночного run. `RunSnapshot` — нормализованные
метрики для A/B-сравнения двух прогонов (с/без skill). Разные назначения,
разные поля. Оставить как есть, но добавить явный конвертер
`artifact.RunBundle → skilldelta.RunSnapshot` в `pkg/skilldelta/extract.go`
(уже частично есть, проверить полноту).

### 4.3 `config.Provider` (string) vs `agent.Provider` (interface) — не дубль

Первое — имя провайдера в конфиге ("bifrost"/"anthropic"). Второе —
runtime-интерфейс. Разные уровни, имя совпадает случайно. Оставить.

---

## 5. Избыточное переусложнение

### 5.1 `pkg/harness/run.go` — 1258 строк, 6 фаз в одной функции — средний

`Harness.Run()` линейно делает: NS cleanup → bootstrap → break inject →
agent execution → verify → artifacts → evidra report → store. В `stages.go`
(163 строки) уже есть зачатки разделения на фазы, но основной путь идёт не
через них.

Последствия. Unit-тестирование отдельных фаз затруднено
(`pkg/harness/run_test.go` существует, но покрывает узкие срезы). Любой
новый режим исполнения (например, `--dry-run-after-verify`) требует
понимания всего файла.

Фикс. Выделить интерфейс `Phase` и несколько реализаций
(`NamespaceCleanupPhase`, `BootstrapPhase`, `BreakPhase`, `AgentPhase`,
`VerifyPhase`, `ReportPhase`). `Harness.Run` становится оркестратором
списка фаз. Оценка: 1–2 дня + тесты. Крупный рефакторинг, делать
отдельным PR.

Консервативный промежуточный шаг. Разнести `executeSingleAgent`, `runWithA2A`,
`runWithProvider`, `applyBreak` в отдельные файлы `agent.go`, `a2a.go`,
`break.go` внутри `pkg/harness/`. Это 15 минут и уже снимает 40% объёма
`run.go`.

### 5.2 Три HTTP-клиента к Anthropic — средний

Файлы:
- `pkg/agent/anthropic.go:17` — прямой клиент Messages API.
- `pkg/agent/bifrost.go:18` — OpenAI-compat клиент (в прод-пути).
- `pkg/agent/claude.go:18` — обёртка над Claude CLI (`exec.Command`).

Каждый имеет собственные retry/backoff (`pkg/agent/retry.go` — общий, но
применяется неоднородно) и свою обработку rate limit. Три реализации
дублируют ~200 строк вспомогательного кода.

Вариант фикса.
- Bifrost — это единственный путь, который используется в проде
  (CLAUDE.md подтверждает: «Все 10 бенчмаркнутых моделей через bifrost»).
- `anthropic.go` — backup для ситуации, когда нет bifrost. Если он не
  используется — удалить. Проверить: `grep -r 'NewAnthropicProvider' cmd/ pkg/`.
- `claude.go` — CLI-путь, нишевой. Оставить, но вынести retry в общий `retry.go`.

Консолидация retry/backoff под общий `retryWithBackoff(ctx, op)` —
полдня работы.

### 5.3 `ProxyEvidenceWriter` — интерфейс на одну реализацию — низкий

Файл: `pkg/agent/tools.go:72`.

Интерфейс с 2 методами (`Prescribe`, `Report`), реализуется единожды
внутри `pkg/agent/smart_prescribe.go:106`. Мотивации для абстракции нет —
нет теста с мок-реализацией, нет второго провайдера.

Фикс. Убрать интерфейс, встроить методы в `SmartToolExecutor` напрямую.
15 минут. Низкий приоритет — живёт, не мешает.

### 5.4 `pkg/adapter` — НЕ мёртвый код

Агент-ревьюер утверждал, что `pkg/adapter` — мёртвый legacy. **Это неверно**:
- `cmd/bench-cli/main.go:631,634,636` — `resolveLocalAdapter` создаёт
  `CLIAdapter`/`MCPAdapter`.
- `pkg/harness/run.go:18` — импортирует `adapter`, передаёт его в `RunRequest.Adapter`.

CLAUDE.md называет это «legacy path», но функционально оба пути живы
и выбираются по флагу `--adapter`. «Legacy» здесь относительно нового
`--provider`-пути, а не абсолютно. Удалять нельзя, пока существует MCP-server-путь
и внешний CLI-путь. Переименовать в CLAUDE.md «legacy» → «adapter-based execution»
во избежание путаницы.

### 5.5 Не подтверждено: `CommandRunner` — интерфейс на одну реализацию

Агент-ревьюер отметил `CommandRunner` как избыточный интерфейс. **Это неверно**:
интерфейс используется в `Bootstrapper`, `LocalProvisioner`, тестах
(`pkg/environment/local_provisioner_test.go:45` имеет `newFakeProvider`,
использующий `CommandRunner`). Классический seam для тестирования exec-вызовов.
Оставить.

---

## 6. Два пути исполнения: sequential vs parallel

Подтверждено существование двух путей:

```
sequential:  CLI → provisioner.Acquire → harness.Run
parallel:    CLI → orchestrator.Provision → jobqueue → worker → workspace + RewriteNamespace → harness.Run
```

Хорошие новости:
- Оба пути сходятся в `harness.Run` — нет двух реализаций цикла сценария.
- `ARCHITECTURE.md` уже подробно описывает parallel-путь (строки 156–194).

Плохие новости:
- `CLAUDE.md` отстал: не упоминает `pkg/orchestrator`, `pkg/workspace`,
  `pkg/jobqueue`. Заметные пакеты, не отражены в источнике правды для агентов.
- `workspace.RewriteNamespace` — текстовая подмена строки `"bench"` на
  `"bench-wN"` в YAML'ах. Хрупко: если в комментарии, имени image-тега,
  label'е или конфиге агента встретится слово `bench` — его перепишут.

Рекомендация по namespace-изоляции. Сделать namespace явным параметром
сценария. Bootstrap-план должен принимать `--namespace` и подставлять его
как параметр в манифесты (`sed s/{{NAMESPACE}}/bench-w0/g`), а не искать
буквальное слово `bench`. Делать отдельным PR, 1 день.

Рекомендация по документации. Синхронизировать CLAUDE.md и ARCHITECTURE.md.
В CLAUDE.md добавить одну секцию «Parallel execution» с указателем
на ARCHITECTURE.md за подробностями. 30 минут.

---

## 7. Поток данных и потеря информации

Единая сборка `agentResult.Metadata` в `harness/run.go:1025` — это хорошо.
Но дальше три sink'а (disk/SQLite/Evidra API) имеют разные окна:

```
agentResult (полный)
   │
   ├── artifact/writer.go     → run.json + tool-calls.json + transcript.txt + prompt.txt + chaos.json  ✓ полный
   ├── store/store.go         → SQLite (metadata_json + checks_json), БЕЗ tool_calls/transcript/prompt  ✗ частичный
   └── report/evidra.go       → POST /v1/bench/runs, transcript + tool_calls отдельными аргументами    ≈ полный, но не prompt/chaos
```

Воспроизводимость run'а требует наличия всех трёх: БД даёт index, artifact'ы —
содержимое, API — центральный учёт. Диск — единственное место с полным payload.
Если диск потерян — восстановить run невозможно даже при наличии БД и API.

Фикс (минимальный для аудита):
- `tool_calls_json`, `transcript` → в `bench.RunRecord` (см. §3.3).
- `prompt`, `chaos_log` → либо в БД, либо ссылкой на `artifacts_dir` (и gc-политика по ней).

---

## 8. План действий по приоритетам

**Быстрые победы (≤ 1 часа каждая):**
1. Объединить `ToolCall` в одно место.
2. Убрать `tui.RunRecord`, читать `artifact.RunBundle`.
3. Переименовать три `RunResult` (косметика).
4. Убрать `ProxyEvidenceWriter` интерфейс.
5. Обновить CLAUDE.md: orchestrator/workspace/jobqueue → указатель на ARCHITECTURE.md.

**Полдня каждая:**
6. Консолидировать retry/backoff в `pkg/agent/retry.go`, применить во всех трёх провайдерах.
7. Разнести `pkg/harness/run.go` по файлам (agent.go, break.go, a2a.go) — промежуточный шаг до Phase-рефакторинга.
8. Решить судьбу `anthropic.go` (удалить или оставить с общим retry).

**1–2 дня каждая:**
9. Добавить `tool_calls_json`, `transcript`, `prompt` в `bench.RunRecord` (согласовать с evidra).
10. Вынести версии в отдельные колонки SQLite, добавить индексы.
11. Превратить `workspace.RewriteNamespace` из текстовой подмены в параметризацию bootstrap-плана.

**Крупный рефакторинг (отдельный PR):**
12. Разбить `harness.Run` на `Phase` с явной оркестрацией.

---

## 9. Что не ломать

- `pkg/scenario` — чистая YAML-доменная модель, не трогать абстракции ради абстракций.
- `pkg/verifier` — каждый `Check` самодостаточен, общий `common.go` выглядит
  как «свалка», но на деле — 6 однотипных проверок по ~50 строк, разносить
  по файлам смысла мало.
- `pkg/environment` — `ClusterLifecycle`/`KindProvider`/`K3dProvider`/`CommandRunner`
  живут правильно, seam для тестов оправдан.
- `pkg/jobqueue` — это тонкая обёртка над River с типизированным
  `BenchJobArgs`, ожидаемая граница.

---

## Приложение А. Методология

Ревью собрано из 4 параллельных проходов по кодовой базе
(Explore-агенты) + ручная верификация каждого нетривиального утверждения
по `grep`/чтению файлов. Утверждения, которые при верификации не
подтвердились, явно отмечены в §5.4 и §5.5.

Источники правды, использованные при ревью:
- `pkg/` — исходный код
- `docs/ARCHITECTURE.md` — актуальная архитектурная документация (swежее, чем CLAUDE.md)
- `CLAUDE.md` — частично устарел в разделе «Packages»

Границы ревью. Не рассматривались: UI/React (`ui/`), скрипты `scripts/`,
CI-конфигурация, тесты самих сценариев. Фокус — Go-пакеты под `pkg/` и `cmd/`.
