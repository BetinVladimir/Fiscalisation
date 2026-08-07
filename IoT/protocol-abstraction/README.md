# Universal Protocol Abstraction Layer

Слой унификации протоколов фискальных устройств.

Расположение в монорепозитории: `IoT/protocol-abstraction`.

## Цель
Преобразование canonical BeeFiscal команд в vendor-specific Daisy 93PC и
Datecs Bulgaria v2.11.4 протоколы для двух разрешённых IoT tracks MVP.

## Функции
- Compile-time выбор подтверждённого device profile; неподтверждённый profile не автоактивируется.
- Маппинг команд и форматов данных.
- Нормализация ответов и ошибок.
- Версионирование адаптеров.

## Модель сборки

- Это C++/Arduino модуль, который собирается вместе с firmware из `IoT/firmware`.
- Целевой протокол выбирается на этапе компиляции через конфигурацию препроцессора.
- Для разных устройств формируются разные firmware-артефакты с разными флагами сборки.

Пример идеи конфигурации (PlatformIO `build_flags`):

```ini
build_flags =
	-DPROTOCOL_EPSON
```

```ini
build_flags =
	-DPROTOCOL_DATECS
```

## Планируемая структура
- `adapters/daisy/`
- `adapters/datecs/`
- `contracts/` — канонические команды/события.

Текущий registry классифицирует все 88 Daisy и 73 Datecs command IDs. Любой
неизвестный код отклоняется, а conditional payment commands 194/55 остаются
`Excluded` до закрытия vendor/acquirer P0. Frame codec реализует wire envelope,
length/BCC/status extraction; command-specific payload/status semantics и HIL
evidence продолжают закрываться отдельно и не считаются production-approved.
