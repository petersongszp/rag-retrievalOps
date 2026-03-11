# Java 基础入门

本文档涵盖 Java 语言基础、面向对象编程、集合框架、异常、IO、JVM 基础等内容。

## 目录
- 语法与类
- 面向对象
- 集合框架
- 异常与处理
- IO 与NIO基础

## 语法与类
Java 强类型、面向对象，跨平台依赖 JVM。

```java
public class Hello {
  public static void main(String[] args) {
    System.out.println("Hello, Java");
  }
}
```

## 面向对象
- 封装、继承、多态；组合优于继承
- 抽象类与接口：一个类可实现多个接口
- 覆盖 `equals/hashCode/toString` 时保持自洽与幂等

## 集合框架
- `List/Set/Map` 及常见实现：`ArrayList`、`LinkedList`、`HashSet`、`LinkedHashSet`、`HashMap`、`LinkedHashMap`、`TreeMap`
- 线程安全：`Collections.synchronizedXxx`、`ConcurrentHashMap`（分段/红黑树化）
- 选择：读多写少→`ConcurrentHashMap`，有序→`LinkedHashMap`/`TreeMap`

## 异常与处理
- 受检异常与非受检异常；边界处转换为业务语义异常
- 不要吞掉异常；记录根因并带上上下文
- `try-with-resources` 自动关闭资源

## IO 与 NIO 基础
- 传统 IO（阻塞流）与 NIO（Channel/Buffer/Selector）
- NIO 适合高并发网络 IO；配合 Netty 进一步抽象

## 泛型与 Stream
- 泛型擦除，使用边界通配：`List<? extends T>`/`<? super T>`
- `Stream` 流式处理与并行流；注意避免在热点路径滥用装箱

```java
int sum = IntStream.of(1,2,3).map(x -> x * 2).sum();
```

## 并发基础
- `Thread`、`ExecutorService`、`ForkJoinPool`
- 同步：`synchronized`、`ReentrantLock`、`CountDownLatch`、`Semaphore`
- `volatile` 与 JMM 可见性；避免双重检查错误用法

## 构建与测试
- 构建：Maven/Gradle；版本约束、依赖冲突解决
- 测试：JUnit5、AssertJ、Mockito；参数化测试

## 最佳实践清单
- 接口小而稳；依赖倒置，面向接口编程
- 合理选择集合类型，避免过早优化
- 关闭 I/O 资源，避免内存/句柄泄漏
- 明确不可变对象与线程安全边界
- 使用错误码+异常的统一错误处理策略


