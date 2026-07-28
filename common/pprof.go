package common

import (
	"context"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/shirou/gopsutil/cpu"
)

// Monitor preserves the legacy blocking entrypoint.
func Monitor() {
	RunPprofMonitor(context.Background())
}

// RunPprofMonitor 定时监控cpu使用率，超过阈值输出pprof文件
func RunPprofMonitor(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		percent, err := cpu.Percent(time.Second, false)
		if err != nil {
			SysLog("读取 CPU 使用率失败 " + err.Error())
			continue
		}
		if len(percent) > 0 && percent[0] > 80 {
			fmt.Println("cpu usage too high")
			// write pprof file
			if _, err := os.Stat("./pprof"); os.IsNotExist(err) {
				err := os.Mkdir("./pprof", os.ModePerm)
				if err != nil {
					SysLog("创建pprof文件夹失败 " + err.Error())
					continue
				}
			}
			f, err := os.Create("./pprof/" + fmt.Sprintf("cpu-%s.pprof", time.Now().Format("20060102150405")))
			if err != nil {
				SysLog("创建pprof文件失败 " + err.Error())
				continue
			}
			err = pprof.StartCPUProfile(f)
			if err != nil {
				SysLog("启动pprof失败 " + err.Error())
				continue
			}
			timer := time.NewTimer(10 * time.Second)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				pprof.StopCPUProfile()
				_ = f.Close()
				return
			case <-timer.C:
			}
			pprof.StopCPUProfile()
			_ = f.Close()
		}
		timer := time.NewTimer(30 * time.Second)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}
