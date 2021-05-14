# blockchair下载器

## 编译

没有跨平台编译需求就直接：

```shell
cd urls/blockchair
go build .
```

## 用法

```shell
# 格式：// blockchair [-n 20] [20210315][-20210320]

# 不加任何flag/arg，则默认下载当天数据（blockchair网没有，因此会报错退出）
blockchair

# 下载 20210315 一天的数据
blockchair 20210315

# 下载 20210315-20210320 多天的数据
blockchair 20210315-20210320

# 指定最大下载器数量100， 下载 20210315 一天的数据
blockchair -n 100 20210315
```

## TODO

- 删除不必要的代码
- 增加日志开关（日志占用太高）
