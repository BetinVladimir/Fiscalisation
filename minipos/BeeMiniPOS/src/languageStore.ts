import AsyncStorage from "@react-native-async-storage/async-storage";

// Metro Web currently emits Zustand's ESM build into a classic script and
// leaves import.meta intact. Requiring the package selects its CJS/React Native
// entrypoint and keeps the generated Expo bundle executable in browsers.
declare const require: (id: string) => unknown;
const { create } = require("zustand") as typeof import("zustand");
const { createJSONStorage, persist } = require("zustand/middleware") as typeof import("zustand/middleware");

export type Language = "bg" | "ru" | "en";

const copy = {
  bg: {
    theme: "Тема", systemTheme: "Системна", lightTheme: "Светла", darkTheme: "Тъмна", language: "Език",
    signIn: "Вход с имейл", email: "Имейл", sendCode: "Изпрати код", oneTimeCode: "Еднократен код", continue: "Продължи",
    companyDetails: "Данни за компанията", companyName: "Наименование", address: "Адрес", taxIdentifier: "Данъчен идентификатор", fullName: "Вашите имена", createCompany: "Създай компания",
    sales: "Продажби", configuration: "Конфигурация", openSales: "Отвори продажбите", openConfiguration: "Отвори конфигурацията",
    logout: "Излез", connectBle: "Свържи BLE", bleConnected: "BLE свързан", bleReady: "BLE връзката е готова",
    reconcileZ: "Свери Z", closeShift: "Затвори смяна", openShift: "Отвори смяна", products: "Продукти", employees: "Служители",
    currentSale: "Текуща продажба", touchProduct: "Докоснете продукт", searchBarcode: "Търсене или сканиране на баркод", vat: "ДДС", total: "ОБЩО",
    cashEur: "В брой EUR", cashCard: "В брой + карта", cash: "В брой", card: "Карта", checkResult: "Провери резултата",
    back: "Назад", newProduct: "Нов продукт", name: "Име", barcode: "Баркод", priceEur: "Цена EUR", save: "Запази",
    taxGroups: "ДДС групи", vatGroup: "ДДС група", newTaxGroup: "Нова ДДС група", editTaxGroup: "Редактиране на ДДС група", taxCode: "Код A–H", taxRate: "Ставка ДДС, %", configureTaxGroups: "Първо създайте ДДС група", active: "активна", inactive: "неактивна",
    newEmployee: "Нов служител", firstName: "Име", lastName: "Фамилия", operatorCode: "Код на оператор",
    reversal: "Сторно на завършена продажба", selectReversal: "Изберете продажба с разрешено действие REVERSE", returnSale: "Сторно • връщане",
    locationWorkstation: "Точка и касово място", locationName: "Име на точката", workstationName: "Име на касата", fiscalRegisterId: "Fiscal register ID", saveConfiguration: "Запиши конфигурацията",
    salesReport: "Търговски отчет", refreshReport: "Обнови отчета", noActions: "няма", registerState: "ФУ/каса", state: "Състояние",
    ready: "Готово", resultChecking: "Резултатът се проверява", doNotRepeatPayment: "Не повтаряйте плащането", allowedActions: "Разрешени действия",
    add: "Добави", decrease: "Намали", increase: "Увеличи", discount10: "Отстъпка 10 процента за", discountEur: "Отстъпка в евро за",
    splitCashAmount: "Сума в брой при разделено плащане", splitPayment: "Разделено плащане в брой и с карта", cashPayment: "Плащане в брой", cardPayment: "Плащане с карта", statusLabel: "Състояние",
    checkFiscalResult: "Провери фискалния резултат без повторно плащане", reverseSale: "Сторно на продажба", product: "Продукт", employee: "Служител", optionalBarcode: "Баркод (по избор)", code4: "Код (4)", addProduct: "Добави продукт", addEmployee: "Добави служител",
    openShiftFailed: "Смяната не може да бъде отворена", shiftOperationFailed: "Операцията със смяната е отказана", configureBeforeShift: "Първо настройте POS конфигурацията", selectEmployeeBeforeShift: "Първо създайте и изберете служител", invalidRegister: "Изберете валидна фискална каса",
  },
  ru: {
    theme: "Тема", systemTheme: "Системная", lightTheme: "Светлая", darkTheme: "Тёмная", language: "Язык",
    signIn: "Вход по email", email: "Email", sendCode: "Отправить код", oneTimeCode: "Одноразовый код", continue: "Продолжить",
    companyDetails: "Данные компании", companyName: "Название", address: "Адрес", taxIdentifier: "Налоговый идентификатор", fullName: "Ваше имя", createCompany: "Создать компанию",
    sales: "Продажи", configuration: "Конфигурация", openSales: "Открыть продажи", openConfiguration: "Открыть конфигурацию",
    logout: "Выйти", connectBle: "Подключить BLE", bleConnected: "BLE подключён", bleReady: "BLE-соединение готово",
    reconcileZ: "Сверить Z", closeShift: "Закрыть смену", openShift: "Открыть смену", products: "Товары", employees: "Сотрудники",
    currentSale: "Текущая продажа", touchProduct: "Нажмите на товар", searchBarcode: "Поиск или сканирование штрихкода", vat: "НДС", total: "ИТОГО",
    cashEur: "Наличные EUR", cashCard: "Наличные + карта", cash: "Наличные", card: "Карта", checkResult: "Проверить результат",
    back: "Назад", newProduct: "Новый товар", name: "Название", barcode: "Штрихкод", priceEur: "Цена EUR", save: "Сохранить",
    taxGroups: "Группы НДС", vatGroup: "Группа НДС", newTaxGroup: "Новая группа НДС", editTaxGroup: "Редактирование группы НДС", taxCode: "Код A–H", taxRate: "Ставка НДС, %", configureTaxGroups: "Сначала создайте группу НДС", active: "активна", inactive: "неактивна",
    newEmployee: "Новый сотрудник", firstName: "Имя", lastName: "Фамилия", operatorCode: "Код оператора",
    reversal: "Сторно завершённой продажи", selectReversal: "Выберите продажу с разрешённым действием REVERSE", returnSale: "Сторно • возврат",
    locationWorkstation: "Торговая точка и касса", locationName: "Название точки", workstationName: "Название кассы", fiscalRegisterId: "ID фискальной кассы", saveConfiguration: "Сохранить конфигурацию",
    salesReport: "Отчёт о продажах", refreshReport: "Обновить отчёт", noActions: "нет", registerState: "ФУ/касса", state: "Состояние",
    ready: "Готово", resultChecking: "Результат проверяется", doNotRepeatPayment: "Не повторяйте оплату", allowedActions: "Разрешённые действия",
    add: "Добавить", decrease: "Уменьшить", increase: "Увеличить", discount10: "Скидка 10 процентов для", discountEur: "Скидка в евро для",
    splitCashAmount: "Сумма наличными при разделённой оплате", splitPayment: "Разделённая оплата наличными и картой", cashPayment: "Оплата наличными", cardPayment: "Оплата картой", statusLabel: "Состояние",
    checkFiscalResult: "Проверить фискальный результат без повторной оплаты", reverseSale: "Сторно продажи", product: "Товар", employee: "Сотрудник", optionalBarcode: "Штрихкод (необязательно)", code4: "Код (4)", addProduct: "Добавить товар", addEmployee: "Добавить сотрудника",
    openShiftFailed: "Не удалось открыть смену", shiftOperationFailed: "Операция со сменой отклонена", configureBeforeShift: "Сначала настройте конфигурацию POS", selectEmployeeBeforeShift: "Сначала создайте и выберите сотрудника", invalidRegister: "Выберите корректную фискальную кассу",
  },
  en: {
    theme: "Theme", systemTheme: "System", lightTheme: "Light", darkTheme: "Dark", language: "Language",
    signIn: "Email sign in", email: "Email", sendCode: "Send code", oneTimeCode: "One-time code", continue: "Continue",
    companyDetails: "Company details", companyName: "Company name", address: "Address", taxIdentifier: "Tax identifier", fullName: "Your name", createCompany: "Create company",
    sales: "Sales", configuration: "Configuration", openSales: "Open sales", openConfiguration: "Open configuration",
    logout: "Log out", connectBle: "Connect BLE", bleConnected: "BLE connected", bleReady: "BLE connection is ready",
    reconcileZ: "Reconcile Z", closeShift: "Close shift", openShift: "Open shift", products: "Products", employees: "Employees",
    currentSale: "Current sale", touchProduct: "Tap a product", searchBarcode: "Search or scan barcode", vat: "VAT", total: "TOTAL",
    cashEur: "Cash EUR", cashCard: "Cash + card", cash: "Cash", card: "Card", checkResult: "Check result",
    back: "Back", newProduct: "New product", name: "Name", barcode: "Barcode", priceEur: "Price EUR", save: "Save",
    taxGroups: "VAT groups", vatGroup: "VAT group", newTaxGroup: "New VAT group", editTaxGroup: "Edit VAT group", taxCode: "Code A–H", taxRate: "VAT rate, %", configureTaxGroups: "Create a VAT group first", active: "active", inactive: "inactive",
    newEmployee: "New employee", firstName: "First name", lastName: "Last name", operatorCode: "Operator code",
    reversal: "Reverse completed sale", selectReversal: "Select a sale with the REVERSE action", returnSale: "Reverse • return",
    locationWorkstation: "Location and workstation", locationName: "Location name", workstationName: "Workstation name", fiscalRegisterId: "Fiscal register ID", saveConfiguration: "Save configuration",
    salesReport: "Sales report", refreshReport: "Refresh report", noActions: "none", registerState: "Fiscal unit/register", state: "State",
    ready: "Ready", resultChecking: "Checking the result", doNotRepeatPayment: "Do not repeat the payment", allowedActions: "Allowed actions",
    add: "Add", decrease: "Decrease", increase: "Increase", discount10: "10 percent discount for", discountEur: "Discount in euros for",
    splitCashAmount: "Cash amount for split payment", splitPayment: "Split cash and card payment", cashPayment: "Cash payment", cardPayment: "Card payment", statusLabel: "Status",
    checkFiscalResult: "Check fiscal result without repeating payment", reverseSale: "Reverse sale", product: "Product", employee: "Employee", optionalBarcode: "Barcode (optional)", code4: "Code (4)", addProduct: "Add product", addEmployee: "Add employee",
    openShiftFailed: "Could not open shift", shiftOperationFailed: "Shift operation was rejected", configureBeforeShift: "Configure the POS first", selectEmployeeBeforeShift: "Create and select an employee first", invalidRegister: "Select a valid fiscal register",
  },
} as const;

export type Translation = (typeof copy)[Language];

type LanguageState = {
  language: Language;
  setLanguage: (language: Language) => void;
};

export const useLanguageStore = create<LanguageState>()(
  persist(
    (set) => ({ language: "bg", setLanguage: (language) => set({ language }) }),
    { name: "beeminipos-language-store", storage: createJSONStorage(() => AsyncStorage) },
  ),
);

export const useTranslation = () => {
  const language = useLanguageStore((state) => state.language);
  return copy[language];
};
