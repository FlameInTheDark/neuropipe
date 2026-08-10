import type { ConfigField, DataField, NodeDefinition, NodePort } from '@/lib/types'

type CatalogLanguage = 'de' | 'fr' | 'ru'

interface CatalogTranslations {
  categories: Record<string, string>
  titles: Record<string, string>
  terms: Record<string, string>
  contentType: string
  description: (name: string) => string
  fieldDescription: (name: string) => string
  placeholder: (name: string) => string
}

// Built-in node metadata is presentation copy. Stable type and pin IDs remain
// untouched, so localization can never change graph persistence or execution.
const catalogs: Record<CatalogLanguage, CatalogTranslations> = {
  de: {
    contentType: 'Inhaltstyp',
    categories: { Triggers: 'Auslöser', Actions: 'Aktionen', Local: 'Lokal', AI: 'KI', Canvas: 'Canvas', Data: 'Daten', Math: 'Mathematik', Flow: 'Ablauf', Chat: 'Chat', Functions: 'Funktionen', Code: 'Code' },
    titles: {
      'trigger:button': 'Schaltflächen-Auslöser', 'trigger:cron': 'Cron-Auslöser', 'trigger:file_watch': 'Dateiüberwachung', 'trigger:hotkey': 'Globaler Hotkey', 'trigger:webhook': 'Lokaler Webhook', 'trigger:chat': 'Chat-Auslöser',
      'action:http': 'HTTP-Anfrage', 'action:file_read': 'Datei lesen', 'action:file_write': 'Datei schreiben', 'action:list_directory': 'Verzeichnis auflisten', 'action:terminal': 'Terminalbefehl ausführen', 'action:notification': 'Desktop-Benachrichtigung', 'action:report': 'Bericht erstellen', 'action:git': 'Git', 'action:subpipeline': 'Pipeline ausführen', 'action:chat_reply': 'An Chat antworten', 'action:chat_status': 'Chat-Status aktualisieren', 'action:javascript': 'JavaScript',
      'llm:prompt': 'LLM-Prompt', 'llm:extract': 'Strukturierte Extraktion', 'llm:boolean': 'LLM-Boolean-Router', 'llm:choice': 'LLM-Auswahl-Router', 'llm:summarize': 'Zusammenfassen', 'llm:agent': 'Agent', 'llm:coding_agent': 'Coding-Agent',
      'visual:comment': 'Kommentar', 'data:constant': 'Konstante', 'data:format_text': 'Text formatieren', 'data:get_field': 'Feld abrufen', 'data:build_object': 'Objekt erstellen', 'data:break_object': 'Objekt aufteilen', 'data:cast': 'Umwandeln', 'data:type_assert': 'Typzusicherung', 'data:json_query': 'JSON abfragen', 'data:equals': 'Gleich', 'data:greater_than': 'Größer als', 'data:json_parse': 'JSON parsen', 'data:get_variable': 'Variable abrufen', 'data:chat_history': 'Chatverlauf lesen', 'data:reroute': 'Umleitung', 'data:array_append': 'An Array anhängen', 'data:array_get': 'Aus Array auswählen', 'data:get_type': 'Typ ermitteln', 'data:length': 'Länge', 'data:base64_encode': 'Base64 kodieren', 'data:base64_decode': 'Base64 dekodieren',
      'math:add': 'Addieren', 'math:subtract': 'Subtrahieren', 'math:multiply': 'Multiplizieren', 'math:divide': 'Dividieren',
      'flow:branch': 'Verzweigung', 'flow:sequence': 'Sequenz', 'flow:for_each': 'Für-jedes-Schleife', 'flow:for_loop': 'For-Schleife', 'flow:while': 'While-Schleife', 'flow:switch': 'Switch', 'flow:do_once': 'Einmal ausführen', 'flow:gate': 'Gate', 'flow:flip_flop': 'FlipFlop', 'flow:multi_gate': 'MultiGate', 'flow:reroute': 'Umleitung', 'flow:break': 'Abbrechen', 'flow:set_variable': 'Variable setzen', 'flow:return': 'Zurückgeben',
      'function:entry': 'Funktionseinstieg', 'function:return': 'Funktionsrückgabe', 'function:input': 'Funktionseingaben', 'function:output': 'Funktionsausgaben',
    },
    terms: { Exec: 'Ausführen', Then: 'Dann', Start: 'Start', Input: 'Eingabe', Output: 'Ausgabe', Payload: 'Nutzdaten', Result: 'Ergebnis', Value: 'Wert', Text: 'Text', Bytes: 'Bytes', Source: 'Quelle', Key: 'Schlüssel', Object: 'Objekt', Equal: 'Gleich', True: 'Wahr', False: 'Falsch', Error: 'Fehler', Condition: 'Bedingung', Selection: 'Auswahl', Default: 'Standard', Array: 'Array', 'Loop Body': 'Schleifenkörper', Completed: 'Abgeschlossen', 'Array Element': 'Array-Element', 'Array Index': 'Array-Index', Index: 'Index', 'First Index': 'Erster Index', 'Last Index': 'Letzter Index', Reset: 'Zurücksetzen', Enter: 'Eingang', Open: 'Öffnen', Close: 'Schließen', Toggle: 'Umschalten', 'Chat ID': 'Chat-ID', 'Chat Run ID': 'Chat-Run-ID', Messages: 'Nachrichten', Status: 'Status', Tools: 'Werkzeuge', 'Button label': 'Schaltflächenname', 'Global hotkey': 'Globaler Hotkey', 'Cron expression': 'Cron-Ausdruck', Timezone: 'Zeitzone', Path: 'Pfad', Files: 'Dateien', Name: 'Name', Size: 'Größe', 'Signing secret': 'Signaturgeheimnis', 'Chat label': 'Chat-Name', URL: 'URL', Method: 'Methode', Body: 'Inhalt', Content: 'Inhalt', Shell: 'Shell', Command: 'Befehl', 'Working directory': 'Arbeitsverzeichnis', Title: 'Titel', Message: 'Nachricht', 'Report title': 'Berichtstitel', Tags: 'Tags', Markdown: 'Markdown', Operation: 'Vorgang', Repository: 'Repository', Pipeline: 'Pipeline', Prompt: 'Prompt', 'Model override': 'Modellüberschreibung', Instructions: 'Anweisungen', 'Fields to extract': 'Zu extrahierende Felder', Question: 'Frage', Options: 'Optionen', 'Maximum turns': 'Maximale Schritte', Task: 'Aufgabe', Workspace: 'Arbeitsbereich', Format: 'Format', Outputs: 'Ausgaben', 'Target type': 'Zieltyp', 'JSON path': 'JSON-Pfad', Type: 'Typ', 'Element Type': 'Element-Typ', Length: 'Länge', 'Variable name': 'Variablenname', Limit: 'Limit', 'Start open': 'Offen starten', Loop: 'Schleife', 'Response body': 'Antwortinhalt', Headers: 'Header', JSON: 'JSON', 'Report ID': 'Bericht-ID', 'Created at': 'Erstellt am', 'Updated at': 'Aktualisiert am', Decision: 'Entscheidung', Choice: 'Auswahl', 'Request headers': 'Anfrage-Header', 'Use custom User-Agent': 'Eigenen User-Agent verwenden', 'User-Agent': 'User-Agent' },
    description: (name) => `Konfigurieren Sie den Knoten „${name}“ für diesen Blueprint-Graphen.`,
    fieldDescription: (name) => `Bekanntes Ergebnisfeld: ${name}.`,
    placeholder: (name) => `${name} eingeben`,
  },
  fr: {
    contentType: 'Type de contenu',
    categories: { Triggers: 'Déclencheurs', Actions: 'Actions', Local: 'Local', AI: 'IA', Canvas: 'Canvas', Data: 'Données', Math: 'Mathématiques', Flow: 'Flux', Chat: 'Discussion', Functions: 'Fonctions', Code: 'Code' },
    titles: {
      'trigger:button': 'Déclencheur de bouton', 'trigger:cron': 'Déclencheur Cron', 'trigger:file_watch': 'Surveillance de fichier', 'trigger:hotkey': 'Raccourci global', 'trigger:webhook': 'Webhook local', 'trigger:chat': 'Déclencheur de discussion',
      'action:http': 'Requête HTTP', 'action:file_read': 'Lire un fichier', 'action:file_write': 'Écrire un fichier', 'action:list_directory': 'Lister le dossier', 'action:terminal': 'Exécuter une commande terminal', 'action:notification': 'Notification de bureau', 'action:report': 'Créer un rapport', 'action:git': 'Git', 'action:subpipeline': 'Exécuter un pipeline', 'action:chat_reply': 'Répondre à la discussion', 'action:chat_status': 'Mettre à jour le statut de discussion', 'action:javascript': 'JavaScript',
      'llm:prompt': 'Invite LLM', 'llm:extract': 'Extraction structurée', 'llm:boolean': 'Routeur booléen LLM', 'llm:choice': 'Routeur de choix LLM', 'llm:summarize': 'Résumer', 'llm:agent': 'Agent', 'llm:coding_agent': 'Agent de code',
      'visual:comment': 'Commentaire', 'data:constant': 'Constante', 'data:format_text': 'Formater le texte', 'data:get_field': 'Lire un champ', 'data:build_object': 'Construire un objet', 'data:break_object': 'Décomposer un objet', 'data:cast': 'Convertir', 'data:type_assert': 'Assertion de type', 'data:json_query': 'Interroger JSON', 'data:equals': 'Égal', 'data:greater_than': 'Supérieur à', 'data:json_parse': 'Analyser JSON', 'data:get_variable': 'Lire une variable', 'data:chat_history': 'Lire l’historique', 'data:reroute': 'Relais', 'data:array_append': 'Ajouter au tableau', 'data:array_get': 'Lire dans un tableau', 'data:get_type': 'Obtenir le type', 'data:length': 'Longueur', 'data:base64_encode': 'Encoder en Base64', 'data:base64_decode': 'Décoder le Base64',
      'math:add': 'Additionner', 'math:subtract': 'Soustraire', 'math:multiply': 'Multiplier', 'math:divide': 'Diviser',
      'flow:branch': 'Branche', 'flow:sequence': 'Séquence', 'flow:for_each': 'Boucle pour chaque élément', 'flow:for_loop': 'Boucle For', 'flow:while': 'Boucle While', 'flow:switch': 'Switch', 'flow:do_once': 'Exécuter une fois', 'flow:gate': 'Porte', 'flow:flip_flop': 'Bascule', 'flow:multi_gate': 'Porte multiple', 'flow:reroute': 'Relais', 'flow:break': 'Interrompre', 'flow:set_variable': 'Définir une variable', 'flow:return': 'Retourner',
      'function:entry': 'Entrée de fonction', 'function:return': 'Retour de fonction', 'function:input': 'Entrées de fonction', 'function:output': 'Sorties de fonction',
    },
    terms: { Exec: 'Exécuter', Then: 'Puis', Start: 'Début', Input: 'Entrée', Output: 'Sortie', Payload: 'Données', Result: 'Résultat', Value: 'Valeur', Text: 'Texte', Bytes: 'Octets', Source: 'Source', Key: 'Clé', Object: 'Objet', Equal: 'Égal', True: 'Vrai', False: 'Faux', Error: 'Erreur', Condition: 'Condition', Selection: 'Sélection', Default: 'Par défaut', Array: 'Tableau', 'Loop Body': 'Corps de boucle', Completed: 'Terminé', 'Array Element': 'Élément de tableau', 'Array Index': 'Index du tableau', Index: 'Index', 'First Index': 'Premier index', 'Last Index': 'Dernier index', Reset: 'Réinitialiser', Enter: 'Entrer', Open: 'Ouvrir', Close: 'Fermer', Toggle: 'Basculer', 'Chat ID': 'ID de discussion', 'Chat Run ID': 'ID d’exécution de discussion', Messages: 'Messages', Status: 'Statut', Tools: 'Outils', 'Button label': 'Libellé du bouton', 'Global hotkey': 'Raccourci global', 'Cron expression': 'Expression Cron', Timezone: 'Fuseau horaire', Path: 'Chemin', Files: 'Fichiers', Name: 'Nom', Size: 'Taille', 'Signing secret': 'Secret de signature', 'Chat label': 'Libellé de discussion', URL: 'URL', Method: 'Méthode', Body: 'Corps', Content: 'Contenu', Shell: 'Shell', Command: 'Commande', 'Working directory': 'Répertoire de travail', Title: 'Titre', Message: 'Message', 'Report title': 'Titre du rapport', Tags: 'Étiquettes', Markdown: 'Markdown', Operation: 'Opération', Repository: 'Dépôt', Pipeline: 'Pipeline', Prompt: 'Invite', 'Model override': 'Remplacement de modèle', Instructions: 'Instructions', 'Fields to extract': 'Champs à extraire', Question: 'Question', Options: 'Options', 'Maximum turns': 'Tours maximum', Task: 'Tâche', Workspace: 'Espace de travail', Format: 'Format', Outputs: 'Sorties', 'Target type': 'Type cible', 'JSON path': 'Chemin JSON', Type: 'Type', 'Element Type': 'Type d’élément', Length: 'Longueur', 'Variable name': 'Nom de variable', Limit: 'Limite', 'Start open': 'Démarrer ouvert', Loop: 'Boucle', 'Response body': 'Corps de réponse', Headers: 'En-têtes', JSON: 'JSON', 'Report ID': 'ID du rapport', 'Created at': 'Créé le', 'Updated at': 'Mis à jour le', Decision: 'Décision', Choice: 'Choix', 'Request headers': 'En-têtes de la requête', 'Use custom User-Agent': 'Utiliser un User-Agent personnalisé', 'User-Agent': 'User-Agent' },
    description: (name) => `Configurez le nœud « ${name} » pour ce graphe Blueprint.`,
    fieldDescription: (name) => `Champ de résultat connu : ${name}.`,
    placeholder: (name) => `Saisissez ${name.toLocaleLowerCase('fr')}`,
  },
  ru: {
    contentType: 'Тип содержимого',
    categories: { Triggers: 'Триггеры', Actions: 'Действия', Local: 'Локальные', AI: 'ИИ', Canvas: 'Холст', Data: 'Данные', Math: 'Математика', Flow: 'Поток', Chat: 'Чат', Functions: 'Функции', Code: 'Код' },
    titles: {
      'trigger:button': 'Триггер-кнопка', 'trigger:cron': 'Cron-триггер', 'trigger:file_watch': 'Наблюдение за файлами', 'trigger:hotkey': 'Глобальная горячая клавиша', 'trigger:webhook': 'Локальный вебхук', 'trigger:chat': 'Триггер чата',
      'action:http': 'HTTP-запрос', 'action:file_read': 'Чтение файла', 'action:file_write': 'Запись файла', 'action:list_directory': 'Список файлов папки', 'action:terminal': 'Запуск команды терминала', 'action:notification': 'Уведомление рабочего стола', 'action:report': 'Создать отчёт', 'action:git': 'Git', 'action:subpipeline': 'Запустить пайплайн', 'action:chat_reply': 'Ответить в чат', 'action:chat_status': 'Обновить статус чата', 'action:javascript': 'JavaScript',
      'llm:prompt': 'LLM-промпт', 'llm:extract': 'Структурированное извлечение', 'llm:boolean': 'Булевый маршрутизатор LLM', 'llm:choice': 'Маршрутизатор выбора LLM', 'llm:summarize': 'Суммаризация', 'llm:agent': 'Агент', 'llm:coding_agent': 'Кодовый агент',
      'visual:comment': 'Комментарий', 'data:constant': 'Константа', 'data:format_text': 'Форматировать текст', 'data:get_field': 'Получить поле', 'data:build_object': 'Собрать объект', 'data:break_object': 'Разобрать объект', 'data:cast': 'Преобразовать тип', 'data:type_assert': 'Проверка типа', 'data:json_query': 'Запрос JSON', 'data:equals': 'Равно', 'data:greater_than': 'Больше чем', 'data:json_parse': 'Разобрать JSON', 'data:get_variable': 'Получить переменную', 'data:chat_history': 'Прочитать историю чата', 'data:reroute': 'Перенаправление', 'data:array_append': 'Добавить в массив', 'data:array_get': 'Взять из массива', 'data:get_type': 'Определить тип', 'data:length': 'Длина', 'data:base64_encode': 'Кодировать Base64', 'data:base64_decode': 'Декодировать Base64',
      'math:add': 'Сложить', 'math:subtract': 'Вычесть', 'math:multiply': 'Умножить', 'math:divide': 'Разделить',
      'flow:branch': 'Ветвление', 'flow:sequence': 'Последовательность', 'flow:for_each': 'Цикл для каждого', 'flow:for_loop': 'Цикл For', 'flow:while': 'Цикл While', 'flow:switch': 'Переключатель', 'flow:do_once': 'Выполнить один раз', 'flow:gate': 'Шлюз', 'flow:flip_flop': 'Триггер', 'flow:multi_gate': 'Мультишлюз', 'flow:reroute': 'Перенаправление', 'flow:break': 'Прервать', 'flow:set_variable': 'Сохранить переменную', 'flow:return': 'Возврат',
      'function:entry': 'Вход функции', 'function:return': 'Выход из функции', 'function:input': 'Входы функции', 'function:output': 'Выходы функции',
    },
    terms: { Exec: 'Выполнение', Then: 'Затем', Start: 'Старт', Input: 'Вход', Output: 'Выход', Payload: 'Данные', Result: 'Результат', Value: 'Значение', Text: 'Текст', Bytes: 'Байты', Source: 'Источник', Key: 'Ключ', Object: 'Объект', Equal: 'Равно', True: 'Да', False: 'Нет', Error: 'Ошибка', Condition: 'Условие', Selection: 'Выбор', Default: 'По умолчанию', Array: 'Массив', 'Loop Body': 'Тело цикла', Completed: 'Завершено', 'Array Element': 'Элемент массива', 'Array Index': 'Индекс массива', Index: 'Индекс', 'First Index': 'Первый индекс', 'Last Index': 'Последний индекс', Reset: 'Сброс', Enter: 'Вход', Open: 'Открыть', Close: 'Закрыть', Toggle: 'Переключить', 'Chat ID': 'ID чата', 'Chat Run ID': 'ID запуска чата', Messages: 'Сообщения', Status: 'Статус', Tools: 'Инструменты', 'Button label': 'Название кнопки', 'Global hotkey': 'Глобальная горячая клавиша', 'Cron expression': 'Cron-выражение', Timezone: 'Часовой пояс', Path: 'Путь', Files: 'Файлы', Name: 'Имя', Size: 'Размер', 'Signing secret': 'Секрет подписи', 'Chat label': 'Название чата', URL: 'URL', Method: 'Метод', Body: 'Тело', Content: 'Содержимое', Shell: 'Оболочка', Command: 'Команда', 'Working directory': 'Рабочая папка', Title: 'Заголовок', Message: 'Сообщение', 'Report title': 'Название отчёта', Tags: 'Теги', Markdown: 'Markdown', Operation: 'Операция', Repository: 'Репозиторий', Pipeline: 'Пайплайн', Prompt: 'Промпт', 'Model override': 'Переопределение модели', Instructions: 'Инструкции', 'Fields to extract': 'Поля для извлечения', Question: 'Вопрос', Options: 'Варианты', 'Maximum turns': 'Максимум шагов', Task: 'Задача', Workspace: 'Рабочая папка', Format: 'Формат', Outputs: 'Выходы', 'Target type': 'Целевой тип', 'JSON path': 'Путь JSON', Type: 'Тип', 'Element Type': 'Тип элемента', Length: 'Длина', 'Variable name': 'Имя переменной', Limit: 'Лимит', 'Start open': 'Открыт при запуске', Loop: 'Цикл', 'Response body': 'Тело ответа', Headers: 'Заголовки', JSON: 'JSON', 'Report ID': 'ID отчёта', 'Created at': 'Создано', 'Updated at': 'Обновлено', Decision: 'Решение', Choice: 'Выбор', 'Request headers': 'Заголовки запроса', 'Use custom User-Agent': 'Использовать свой User-Agent', 'User-Agent': 'User-Agent' },
    description: (name) => `Настройте узел «${name}» для этого Blueprint-графа.`,
    fieldDescription: (name) => `Известное поле результата: ${name}.`,
    placeholder: (name) => `Введите «${name}»`,
  },
}

function localizedText(translations: CatalogTranslations, value: string): string {
  if (value === 'Content type') return translations.contentType
  return translations.terms[value] ?? value
}

function localizePort(translations: CatalogTranslations, pin: NodePort): NodePort {
  return {
    ...pin,
    label: localizedText(translations, pin.label),
    fields: pin.fields?.map((field) => localizeDataField(translations, field)),
  }
}

function localizeDataField(translations: CatalogTranslations, field: DataField): DataField {
  const label = localizedText(translations, field.label || field.path)
  return { ...field, label, description: translations.fieldDescription(label) }
}

function localizeField(translations: CatalogTranslations, field: ConfigField): ConfigField {
  const label = localizedText(translations, field.label)
  return { ...field, label, placeholder: field.placeholder ? translations.placeholder(label) : field.placeholder, options: field.options?.map((option) => ({ ...option, label: localizedText(translations, option.label) })) }
}

/** Localizes only first-party catalog presentation metadata. Plugin and user-created
 * function data intentionally retains the author-supplied language. */
export function localizeNodeDefinitions(definitions: readonly NodeDefinition[], language: string): NodeDefinition[] {
  const translations = catalogs[language as CatalogLanguage]
  if (!translations) return [...definitions]
  return definitions.map((definition) => {
    if (definition.source !== 'builtin') return definition
    const label = translations.titles[definition.type] ?? definition.label
    return {
      ...definition,
      category: translations.categories[definition.category] ?? definition.category,
      label,
      description: translations.description(label),
      inputs: definition.inputs.map((pin) => localizePort(translations, pin)),
      outputs: definition.outputs.map((pin) => localizePort(translations, pin)),
      fields: definition.fields.map((field) => localizeField(translations, field)),
    }
  })
}
