# Message Brokers And Streaming

Этот раздел нужен для понимания асинхронной обработки и интеграций.

Темы:
- RabbitMQ, Kafka, NATS, Redis Streams, SQS/SNS;
- at-most-once, at-least-once, effectively-once;
- ordering, partitions, consumer groups;
- retries, DLQ, poison messages;
- backpressure и flow control;
- exactly-once claims и их реальные ограничения;
- outbox/inbox pattern;
- schema evolution и contract compatibility.

Что важно уметь объяснить:
- почему Kafka и RabbitMQ решают разные классы задач;
- когда нужен стриминг, а когда обычная очередь;
- как проектировать consumer, чтобы он был идемпотентным.
