# Создание первого плагина

В этом руководстве создаётся минимальный бандл, который Neuropipe обнаружит, а
его Markdown-страница появится в Документации. Оно показывает поддерживаемую
сейчас поверхность v1: обнаружение, диагностику и документацию. Исполняемый
узел холста пока не создаётся.

## Что нужно

- Neuropipe с разделом **Настройки → Расширения**.
- Go 1.26 или совместимая Go-цепочка.
- Доступный для записи корневой каталог плагинов из настроек.

## 1. Создайте каталоги

~~~text
acme-status/
  plugin.json
  sidecar/
    main.go
  docs/
    status-check.md
~~~

Относительные пути разрешаются от каталога с plugin.json.

## 2. Соберите безопасный sidecar

Создайте sidecar/main.go:

~~~go
package main

func main() {
    select {}
}
~~~

Из каталога бандла:

~~~powershell
go build -o sidecar.exe ./sidecar
~~~

Этот sidecar намеренно ничего не делает. Neuropipe пока проверяет только его
наличие и не запускает. Настоящий sidecar в будущем будет использовать
управляемый отменяемый gRPC-жизненный цикл из [системы
плагинов](docs:extensions/plugin-system).

## 3. Создайте plugin.json

~~~json
{
  "id": "acme-status",
  "name": "Acme Status",
  "version": "0.1.0",
  "description": "Пример плагина статуса.",
  "apiVersion": "v1",
  "executable": "sidecar.exe",
  "nodes": [
    {
      "id": "status-check",
      "kind": "action",
      "label": "Status Check",
      "description": "Декларация примера, сейчас не выполняется.",
      "icon": "heart-pulse",
      "color": "#60a5fa",
      "capabilities": ["network"],
      "outputs": [{"id": "result", "label": "Result", "kind": "data"}],
      "fields": [{"name": "url", "label": "URL", "kind": "string", "required": true, "secret": false}]
    }
  ],
  "documentation": [
    {
      "id": "status-check",
      "title": "Status Check",
      "categoryPath": ["Extensions", "Acme Status"],
      "path": "docs/status-check.md"
    }
  ]
}
~~~

Обязательны id, name, apiVersion v1 и executable. Version и description улучшают
диагностику. Fields, outputs и capabilities показывают будущую форму, но пока
не создают узел в Библиотеке.

## 4. Напишите страницу

Создайте docs/status-check.md:

~~~markdown
# Status Check

Этот пример добавляет безопасную локальную Markdown-страницу.

## Пример

Status Check → Create Report
~~~

Путь должен оставаться внутри бандла, а файл — быть не больше 1 MiB.

## 5. Повторно обнаружьте бандл

1. Откройте **Настройки → Расширения**.
2. Проверьте корневой каталог плагинов.
3. Нажмите **Повторно обнаружить плагины**.
4. Убедитесь, что Acme Status показывает версию 0.1.0, один объявленный узел и
   **Healthy**.
5. Откройте **Документация → Расширения → Acme Status → Status Check**.

После изменения манифеста или документации повторите обнаружение.

## Поиск проблем

| Симптом | Решение |
| --- | --- |
| Нет строки | Исправьте корневой каталог и повторите обнаружение. |
| Invalid manifest | Исправьте JSON, id/name или v1. |
| Sidecar unavailable | Соберите sidecar.exe или исправьте путь. |
| Healthy без страницы | Проверьте путь .md, categoryPath, id и размер. |
| Нет узла в Библиотеке | Это ожидаемо: v1 пока не регистрирует и не выполняет узлы плагинов. |

## Пример с двумя узлами

Для бандла с несколькими узлами используйте стабильную идентичность, например `weather-tools`, и объявите как минимум `convert-temperature` и `classify-temperature`. Первый узел выдаёт ключ данных `fahrenheit`; второй позже читает его и выдаёт `band`.

~~~json
"nodes": [
  { "id": "convert-temperature", "kind": "action", "outputs": [{ "id": "fahrenheit", "label": "Fahrenheit", "kind": "data" }], "fields": [] },
  { "id": "classify-temperature", "kind": "action", "outputs": [{ "id": "band", "label": "Band", "kind": "data" }], "fields": [{ "name": "warmAtOrAbove", "label": "Warm at or above", "kind": "number", "required": false, "secret": false }] }
]
~~~

В будущем Go-коде `Validate` проверяет пороги до выполнения. `ConvertTemperature.Execute` возвращает `map[string]any{"fahrenheit": ...}`; `ClassifyTemperature.Execute` читает это значение и возвращает `map[string]any{"band": ...}`. Синхронизируйте ключи выходов и идентификаторы манифеста, а общую проверку чисел вынесите в helper. Текущий runtime v1 пока не запускает и не вызывает эти handler'ы.

Подробнее: [Система плагинов](docs:extensions/plugin-system).
