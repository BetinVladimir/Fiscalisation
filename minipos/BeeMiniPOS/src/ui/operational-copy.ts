// EN MVP operational copy generated using translategemma:12b and reviewed. Other locales retain existing copy; full operational localization remains OPEN.
const english:Record<string,string>={
  "Зареждане…": "Loading...",
  "Готово • отворете смяна": "Ready • open shift",
  "Отворената смяна е възстановена • проверете готовността на ФУ": "Open shift restored • check fiscal device readiness",
  "Влезте с персоналния си профил": "Log in with your personal profile",
  "Смяната е отворена • проверете готовността на ФУ": "Shift open • check fiscal device readiness",
  "Артикулът е приет • фискална продажба ще бъде подготвена при плащане": "Item accepted • a fiscal sale will be prepared at payment",
  "Продажбата е блокирана: няма активна отворена смяна": "Sale blocked: no active open shift",
  "Операторската идентичност не е привързана • продажбата е блокирана": "Operator identity not linked • sale blocked",
  "MiniPOS API е недостъпен • продажбата е блокирана": "MiniPOS API is unavailable • sale blocked",
  "Затворете смяната преди изход": "Close the shift before signing out",
  "Настройте POS преди първата продажба": "Configure POS before the first sale",
  "Създайте служител преди отваряне на смяна": "Create an employee before opening a shift",
  "Настройте валидна фискална каса преди възстановяване на смяна": "Configure a valid fiscal register before restoring a shift",
  "Z-отчетът изисква сверяване • продажбите са блокирани": "Z-report requires reconciliation • sales are blocked",
  "Фискализация…": "Cash payment and fiscalization...",
  "Картово плащане и фискализация…": "Card payment and fiscalization...",
  "Разделено плащане: в брой, карта и фискализация…": "Split payment: cash, card, and fiscalization..."
};
export function operationalLabel(language:string,value:string){if(language!=="en")return value;return english[value]??value.replace(/^Смяната е затворена • Z /,"Shift closed \u2022 Z ").replace(/^Успешен фискален бон • /,"Successful fiscal receipt \u2022 ");}
