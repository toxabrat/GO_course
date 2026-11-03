# GO_course

Домашние задания курса по Go и Python.

## HW1

- [Броски костей](homeworks/hw1/dice)
- [Обработка ошибок](homeworks/hw1/handler)
- [Агрегация HTTP-статусов](homeworks/hw1/urls)

Проверка (Go 1.27+):

```sh
go test -race ./homeworks/hw1/...
```

Пример запуска:

```sh
go run ./homeworks/hw1/dice --seed 42 -n 3 -v 2d10
```
