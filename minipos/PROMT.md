Для 
/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/miniposweb
/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS
одинаково

Добавь авторизацию в виде email и потом динамический пароль
Сохраняй в localctorage токены

Добавь анбординг компании
Как и авторизация но потом еще фрома с данными о компании:
Название
адрес
НАлоговый идентификатор
ФИО текущего пользователя

Добавляй пользователя

Редактирование товаров:
Добавь список всех отдельным экраном
На него перенеси форму добавления как + и новый экран

Для сотрудников 
Добавь список всех отдельным экраном
На него перенеси форму добавления как + и новый экран

Все необходимые эндпониты добавь в /Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend


SMS отправляй через 
rabbit mq из fiscalisation

обработчик в 
/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend

SMTP_HOST = smtp-relay.brevo.com
SMTP_USER = a4d164001@smtp-brevo.com
SMTP_PASSWORD = <runtime secret; rotate the previously exposed credential>
SMTP_MAILDOMAIN = mail.beeloy.com
SMTP_PORT = 587
SMTP_FROM = noreply@beeloy.com

CSS темы поменяй, что бы все цвета были в постельном тоне
Добавь мультиязычность и выбор языка
