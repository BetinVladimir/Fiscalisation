// EN source; existing BG wording; RU generated with translategemma:12b (register corrected to fiscal context).
// Other locales use EN until operational copy translation is completed. Auth copy remains25 locales.
export const shellCopies = {
  "en": {
    "devices": "Devices",
    "operations": "Operations",
    "reports": "Reports",
    "audit": "Audit",
    "administration": "Administration",
    "settings": "Settings",
    "open": "Open {name}",
    "company": "Company {name}",
    "logout": "Sign out",
    "logoutLabel": "Sign out of the administrator account",
    "register": "Register"
  },
  "bg": {
    "devices": "Устройства",
    "operations": "Операции",
    "reports": "Отчети",
    "audit": "Одит",
    "administration": "Администриране",
    "settings": "Настройки",
    "open": "Отвори {name}",
    "company": "Компания {name}",
    "logout": "Излез",
    "logoutLabel": "Излез от административния профил",
    "register": "Касово място"
  },
  "ru": {
    "devices": "Устройства",
    "operations": "Операции",
    "reports": "Отчеты",
    "audit": "Аудит",
    "administration": "Администрирование",
    "settings": "Настройки",
    "open": "Открыть {name}",
    "company": "Компания {name}",
    "logout": "Выйти",
    "logoutLabel": "Выйти из учетной записи администратора",
    "register": "Касса"
  }
} as const;
export function shellCopy(language:string){ return shellCopies[language as keyof typeof shellCopies] ?? shellCopies.en; }
export const shellTabKeys = {
  "Устройства": "devices",
  "Операции": "operations",
  "Отчети": "reports",
  "Одит": "audit",
  "Администриране": "administration",
  "Настройки": "settings"
} as const;
export function shellTabLabel(language:string, tab:string){ const key=shellTabKeys[tab as keyof typeof shellTabKeys]; return key ? shellCopy(language)[key] : tab; }
