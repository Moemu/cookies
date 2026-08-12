// cookies-maintain 跑一次性的维护动作。
//
// 仓里没有定时任务框架，也不该为了一个清理动作引入调度库。所以这些动作先做成
// 手工命令：需要的时候跑一次，看得见删了多少条。要自动化的时候，外面挂一个
// 系统级定时器调它就行，不必改代码。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/database"
	"github.com/shikanon/cookies/internal/systems/insights"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: cookies-maintain <purge-empty-drafts>")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.MySQL)
	if err != nil {
		log.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()

	switch command := os.Args[1]; command {
	case "purge-empty-drafts":
		// 「记一笔」建了但人什么都没记的复盘草稿。留着会一直占住
		// (项目 + 窗口) 的唯一键，下一次记一笔就撞在这份空壳上。
		service := insights.Service{Repository: insights.MySQLRepository{DB: db}}
		purged, err := service.PurgeEmptyDrafts(ctx)
		if err != nil {
			log.Fatalf("purge empty insight report drafts: %v", err)
		}
		log.Printf("清掉 %d 份空的复盘草稿", purged)
	default:
		fmt.Fprintf(os.Stderr, "未知命令 %q\n", command)
		os.Exit(2)
	}
}
