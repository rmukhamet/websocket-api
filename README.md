# Websocket Service for Transport Management System
## Описание
получает сообщения из mq_service (NATS) topic notifications  и отправляет сообщение в websocket пользователя

- при подключении к websocket клиент отправляет jwt token
- сервис отправляет jwt token на api_service (service JWT метод decode) и получает id пользователя и id компании или невалидный токен
- пользователь если принял сообщение отправляет ok
- сервис отправляет подверждение получения сообщения пользователем

### формат принимаемого сообщения

```json5
{
    type: string,
    title: string,
    content: string,
    from: id or null (если null то системное сообщение),
    to: person_id | org_id | null (если null то всем),
}
```

### формат сообщения отправляемого в websocket

```
{"type":"pbx","title":"Звонок","content":"79638966445 звонит на 202"} ? 
{type: "pbx", title: "Звонок", content: "79638966445 звонит на 202", from: undefined, to: undefined} ? 
```
