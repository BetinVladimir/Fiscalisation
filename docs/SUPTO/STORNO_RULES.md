# STORNO_RULES — Правила сторно по чл. 31 Наредба № Н-18

Источник: Наредба № Н-18 от 13 декември 2006 г. (посл. изм. ДВ бр. 2/2024), чл. 31.

| id | алинея | причина | поведение системы | статус |
|---|---|---|---|---|
| SR-01 | 31(1) | OPERATOR_ERROR | Сторно допустимо до края на 7-ия ден от следващия месец (Europe/Sofia) | IMPLEMENTED |
| SR-02 | 31(1) | CUSTOMER_RETURN | Сторно допустимо без срок при събитие-инициатор | IMPLEMENTED |
| SR-03 | 31(1) | CUSTOMER_COMPLAINT | Сторно допустимо без срок при събитие-инициатор | IMPLEMENTED |
| SR-04 | 31(1) | TAX_BASE_REDUCTION | Сторно допустимо без срок при събитие-инициатор | IMPLEMENTED |
| SR-05 | 31(3) | FU_CASH_REFUSAL | ФУ отказва сторно при недостатъчна наличност; системата записва FAILED, не UNKNOWN | GAP |
| SR-06 | 31(3а) | CARD_REFUND | Връщане по карта само след потвърдено фискално сторно | GAP |
| SR-07 | 31(4) | CARD_REFUND_VIA_FU | Сторно-документ с УНП за карта; ФУ генерира референция | GAP |
| SR-08 | 31(7) | EXCEPTION_FORCE_MAJEURE | Сторно без ФУ при форсмажор; изисква административен документ | GAP |
| SR-09 | 31(8) | EXCEPTION_FU_FAILURE | Сторно без ФУ при повреда; изисква протокол | GAP |
| SR-10 | 31(11) | SPECIAL_UNP_STORNO | УНП на сторно-документ = специален префикс + УНП на оригинала | GAP |
| SR-11 | 31(12) | SPECIAL_UNP_CORRECTION | УНП на корекция при грешна данъчна основа | GAP |

## Покрити тестове

- SR-01: `TestOperatorErrorReversalDeadlineUsesSofiaCalendar`, `TestOperatorErrorDeadlineHandlesYearAndDSTBoundaries`
- SR-02..04: `TestReversalReasonsFailClosed`
- SR-05..11: GAP — не са реализирани в reversal_policy.go
