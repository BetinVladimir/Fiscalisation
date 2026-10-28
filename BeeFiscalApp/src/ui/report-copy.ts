// BG source; EN translated by translategemma:12b, reviewed 2026-10-20. Other locales use EN; full operational localization remains OPEN.
const copies={
  "bg": {
    "heading": "Команда към фискалното устройство",
    "intro": "Показват се данните от фискалното устройство.",
    "create": "Създай {type} отчет",
    "auditor": "Преглед само за одитор; създаването на отчет изисква SUPERVISOR или ADMIN.",
    "loading": "Зареждане на отчети",
    "retry": "Опитай отново",
    "empty": "Няма отчети за избрания филтър.",
    "roleError": "Командата за отчет изисква роля SUPERVISOR или ADMIN",
    "registerError": "Командата за отчет изисква валиден Fiscal register UUID",
    "pending": "{type} отчет…",
    "commandError": "Създаването на отчета не успя. Опитайте отново.",
    "loadError": "Данните не могат да се заредят."
  },
  "en": {
    "heading": "Fiscal device report",
    "intro": "Displays data from the fiscal device.",
    "create": "Create {type} report",
    "auditor": "View only for auditor; creating a report requires SUPERVISOR or ADMIN.",
    "loading": "Loading reports",
    "retry": "Retry",
    "empty": "No reports for the selected filter.",
    "roleError": "The report command requires the SUPERVISOR or ADMIN role",
    "registerError": "The report command requires a valid Fiscal register UUID",
    "pending": "{type} report...",
    "commandError": "Failed to create the report. Please try again.",
    "loadError": "Unable to load data."
  }
} as const;
export const reportCopy=(language:string)=>language==='bg'?copies.bg:copies.en;
