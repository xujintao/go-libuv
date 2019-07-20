## golang协程异步io复用模型
每个fd的封装类型都需要组合internal/poll.FD类型，然后由后者去做io  
对于进程继承来的标准fd(0,1,2)，先包装一下，然后internal/poll.FD依然使用阻塞方式去io  
对于自己打开的fd，也是先包装一下，然后internal/poll.FD统一使用异步复用方式进行io，golang使用协程把它模拟成阻塞方式  
```
listener/conn-----net.(*netFD)-------
                                     |-------internal/poll.(*FD)--------runtime跨平台支持poll
                  os.(*file) --------        (支持blocking和poll)
```

## libuv事件驱动异步io复用模型
```
listener/conn--------libuv(原则上也应该支持跨平台，现在只支持linux)
```