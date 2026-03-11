package alert

import "log"

// 12.2 告警与通知系统：统一入口
// 12.2.1 飞书通知集成：通过 SendFeishuAlert / SendFeishuCard 及下方封装对外提供
// 12.2.2 系统异常与业务风控告警：使用 SystemException / BusinessRisk 上报

// SystemException 上报系统异常（如 Eino 组件错误、超时、Panic）
// 会发送飞书红色卡片告警，若未配置飞书则仅打日志
func SystemException(traceID, component, errMsg string) {
	go func() {
		if err := SendSystemExceptionAlert(traceID, component, errMsg); err != nil {
			logIfNeeded("SystemException", err)
		}
	}()
}

// BusinessRisk 上报业务风控（如敏感内容、滥用、配额超限）
// 会发送飞书橙色卡片告警
func BusinessRisk(reason, detail string) {
	go func() {
		if err := SendBusinessRiskAlert(reason, detail); err != nil {
			logIfNeeded("BusinessRisk", err)
		}
	}()
}

func logIfNeeded(scope string, err error) {
	if err != nil {
		log.Printf("[Alert] %s 发送失败: %v", scope, err)
	}
}
