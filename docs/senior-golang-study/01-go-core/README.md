# Go Core

Сюда складывай материалы по самому языку и runtime.

Темы:
- указатели, value vs reference semantics;
- интерфейсы, method sets, nil interface pitfalls;
- generics и ограничения по их применению;
- ошибки, wrapping, sentinel errors, `errors.Is` и `errors.As`;
- контексты, cancellation, deadlines, propagation;
- modules, versioning, replace, workspace mode;
- escape analysis, stack vs heap;
- garbage collector и влияние allocation rate;
- scheduler, `GOMAXPROCS`, preemption;
- memory model, happens-before, visibility between goroutines.

Вопросы для senior-уровня:
- почему горутина может "утечь" и как это заметить;
- чем отличается `nil` интерфейс от интерфейса с `nil` внутри;
- когда generics полезнее интерфейсов, а когда нет;
- как читать вывод `go build -gcflags=-m`;
- какие anti-patterns чаще всего встречаются в production Go code.
