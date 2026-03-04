# 性能基线 (Benchmark Baseline)

> **用途**：本文件记录 sync.Pool 与调度表（dispatch table）优化 **前** 的分配基线。  
> 用于对比优化后的改善效果。

- **日期**：2026-03-04  
- **Go 版本**：go1.25.6 windows/amd64  
- **CPU**：Intel(R) Core(TM) i5-14400  
- **OS**：Windows (GOOS=windows, GOARCH=amd64)  
- **测试命令**：`go test -bench=. -benchmem -count=3 -benchtime=1s ./routines/node/`

---

## 原始输出

```
goos: windows
goarch: amd64
pkg: github.com/gmbytes/snow/routines/node
cpu: Intel(R) Core(TM) i5-14400
BenchmarkMessageMarshal_Request-16            	 3270045	       367.3 ns/op	     376 B/op	       5 allocs/op
BenchmarkMessageMarshal_Request-16            	 3287446	       431.3 ns/op	     376 B/op	       5 allocs/op
BenchmarkMessageMarshal_Request-16            	 1473513	       808.5 ns/op	     376 B/op	       5 allocs/op
BenchmarkMessageMarshal_Response-16           	 2718238	       437.8 ns/op	     176 B/op	       5 allocs/op
BenchmarkMessageMarshal_Response-16           	 2911620	       401.9 ns/op	     176 B/op	       5 allocs/op
BenchmarkMessageMarshal_Response-16           	 2695198	       409.5 ns/op	     176 B/op	       5 allocs/op
BenchmarkMessageUnmarshal-16                  	248874980	         4.745 ns/op	       0 B/op	       0 allocs/op
BenchmarkMessageUnmarshal-16                  	250012448	         4.787 ns/op	       0 B/op	       0 allocs/op
BenchmarkMessageUnmarshal-16                  	249299468	         4.787 ns/op	       0 B/op	       0 allocs/op
BenchmarkMessageMarshalArgs-16                	 2791804	       430.6 ns/op	     104 B/op	       3 allocs/op
BenchmarkMessageMarshalArgs-16                	 2777916	       429.4 ns/op	     104 B/op	       3 allocs/op
BenchmarkMessageMarshalArgs-16                	 2936865	       427.5 ns/op	     104 B/op	       3 allocs/op
BenchmarkMessageUnmarshalArgs-16              	 1528296	       740.4 ns/op	     200 B/op	       7 allocs/op
BenchmarkMessageUnmarshalArgs-16              	 1340524	       830.2 ns/op	     200 B/op	       7 allocs/op
BenchmarkMessageUnmarshalArgs-16              	 1451305	       840.2 ns/op	     200 B/op	       7 allocs/op
BenchmarkNewRpcContext-16                     	 3576868	       343.5 ns/op	     176 B/op	       3 allocs/op
BenchmarkNewRpcContext-16                     	 3729988	       343.3 ns/op	     176 B/op	       3 allocs/op
BenchmarkNewRpcContext-16                     	 3450630	       338.4 ns/op	     176 B/op	       3 allocs/op
BenchmarkNewPromise-16                        	1000000000	         0.3987 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewPromise-16                        	1000000000	         0.3860 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewPromise-16                        	1000000000	         0.3881 ns/op	       0 B/op	       0 allocs/op
BenchmarkReflectCall-16                       	 1649685	       720.1 ns/op	      16 B/op	       1 allocs/op
BenchmarkReflectCall-16                       	 1684634	       726.6 ns/op	      16 B/op	       1 allocs/op
BenchmarkReflectCall-16                       	 1684989	       707.7 ns/op	      16 B/op	       1 allocs/op
BenchmarkDirectCall-16                        	236429162	         5.001 ns/op	       0 B/op	       0 allocs/op
BenchmarkDirectCall-16                        	239226484	         5.069 ns/op	       0 B/op	       0 allocs/op
BenchmarkDirectCall-16                        	234642271	         4.998 ns/op	       0 B/op	       0 allocs/op
BenchmarkJsonCodecMarshal-16                  	 2348259	       520.7 ns/op	     120 B/op	       2 allocs/op
BenchmarkJsonCodecMarshal-16                  	 2357598	       514.4 ns/op	     120 B/op	       2 allocs/op
BenchmarkJsonCodecMarshal-16                  	 2360572	       514.5 ns/op	     120 B/op	       2 allocs/op
BenchmarkJsonCodecUnmarshal-16                	  843477	      1475 ns/op	     632 B/op	      20 allocs/op
BenchmarkJsonCodecUnmarshal-16                	  854907	      1511 ns/op	     632 B/op	      20 allocs/op
BenchmarkJsonCodecUnmarshal-16                	  792292	      1441 ns/op	     632 B/op	      20 allocs/op
BenchmarkServiceEntry-16                      	  866100	      1327 ns/op	     408 B/op	       8 allocs/op
BenchmarkServiceEntry-16                      	  925946	      1363 ns/op	     408 B/op	       8 allocs/op
BenchmarkServiceEntry-16                      	  936037	      1304 ns/op	     408 B/op	       8 allocs/op
BenchmarkServiceEntry_WithSerialization-16    	  525759	      2278 ns/op	     552 B/op	      14 allocs/op
BenchmarkServiceEntry_WithSerialization-16    	  514186	      2274 ns/op	     552 B/op	      14 allocs/op
BenchmarkServiceEntry_WithSerialization-16    	  495998	      2297 ns/op	     552 B/op	      14 allocs/op
PASS
ok  	github.com/gmbytes/snow/routines/node	62.039s
```

---

## 汇总表（优化前基线）

| 基准测试 | 分类 | 均值耗时 | B/op | allocs/op | 说明 |
|---------|------|---------|------|-----------|------|
| `BenchmarkMessageMarshal_Request` | 序列化 | ~536 ns | 376 | 5 | 请求消息序列化：fName + 3个参数 |
| `BenchmarkMessageMarshal_Response` | 序列化 | ~416 ns | 176 | 5 | 响应消息序列化：2个返回值 |
| `BenchmarkMessageUnmarshal` | 反序列化 | ~4.8 ns | 0 | 0 | 仅解析消息头，无分配 |
| `BenchmarkMessageMarshalArgs` | 序列化 | ~429 ns | 104 | 3 | 单独测量 marshalArgs |
| `BenchmarkMessageUnmarshalArgs` | 反序列化 | ~804 ns | 200 | 7 | 单独测量 unmarshalArgs |
| `BenchmarkNewRpcContext` | 上下文创建 | ~342 ns | 176 | 3 | rpcContext + context.WithCancel |
| `BenchmarkNewPromise` | Promise创建 | ~0.39 ns | 0 | 0 | promise 逃逸分析已优化 |
| `BenchmarkReflectCall` | 反射调度 | ~718 ns | 16 | 1 | reflect.Value.Call 调度开销 |
| `BenchmarkDirectCall` | 直接调用（对照） | ~5.0 ns | 0 | 0 | 直接函数调用基线 |
| `BenchmarkJsonCodecMarshal` | 编解码 | ~516 ns | 120 | 2 | JSON 序列化 |
| `BenchmarkJsonCodecUnmarshal` | 编解码 | ~1476 ns | 632 | 20 | JSON 反序列化 **分配热点** |
| `BenchmarkServiceEntry` | 完整路径 | ~1331 ns | 408 | 8 | 本地调用完整路径（无序列化） |
| `BenchmarkServiceEntry_WithSerialization` | 完整路径 | ~2283 ns | 552 | 14 | 远程调用完整路径（含反序列化） |

---

## 关键发现

### 1. 反射调度开销巨大（144x）

`BenchmarkReflectCall`（718 ns）vs `BenchmarkDirectCall`（5 ns）= **144 倍开销**。

反射调用每次有 **1 allocs/op（16 B）**，而直接调用为零分配。  
这是调度表（dispatch table）优化的主要收益来源。

### 2. rpcContext 每次 RPC 创建 3 次分配

`BenchmarkNewRpcContext` = 176 B / 3 allocs（rpcContext 结构体 + context + cancel func）。  
`sync.Pool` 复用可将这部分降至 0 allocs/op。

### 3. JSON 反序列化是最大分配热点

`BenchmarkJsonCodecUnmarshal` = **632 B / 20 allocs** — 远超其他路径。  
远程调用的 `BenchmarkServiceEntry_WithSerialization` 多出的 144 B / 6 allocs 主要来自此处。

### 4. 本地调用完整路径（Entry）= 8 allocs

`BenchmarkServiceEntry`（408 B / 8 allocs）分解：
- rpcContext 创建：~176 B / 3 allocs  
- reflect.Call fArgs slice：~16 B / 1 alloc  
- Return/flush 路径：~216 B / 4 allocs  

`sync.Pool` 复用 rpcContext 和 fArgs slice 可显著降低此数值。

---

## 优化目标

| 优化方向 | 目标基准 | 期望改善 |
|---------|---------|---------|
| `sync.Pool` 复用 rpcContext | `BenchmarkNewRpcContext` | 3 allocs → 0 allocs |
| `sync.Pool` 复用 fArgs slice | `BenchmarkReflectCall` | 1 alloc → 0 allocs |
| 调度表替换反射 | `BenchmarkReflectCall` vs `BenchmarkDirectCall` | 718 ns → 接近 5 ns |
| 完整路径综合 | `BenchmarkServiceEntry` | 8 allocs → ≤2 allocs |
