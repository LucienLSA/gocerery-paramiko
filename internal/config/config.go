// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import "github.com/zeromicro/go-zero/rest"

// 配置文件结构体
type Config struct {
	rest.RestConf
	Bastion  BastionConfig  `json:"Bastion" yaml:"Bastion" mapstructure:"Bastion"`
	Targets  []TargetConfig `json:"Targets" yaml:"Targets" mapstructure:"Targets"`
	Executor ExecutorConfig `json:"Executor" yaml:"Executor" mapstructure:"Executor"`
	Celery   CeleryConfig   `json:"Celery" yaml:"Celery" mapstructure:"Celery"`
}

// 跳板机配置
type BastionConfig struct {
	Host     string `json:"Host" yaml:"Host" mapstructure:"Host"`
	Port     int    `json:"Port" yaml:"Port" mapstructure:"Port"`
	User     string `json:"User" yaml:"User" mapstructure:"User"`
	Password string `json:"Password" yaml:"Password" mapstructure:"Password"`
}

// 目标主机配置
type TargetConfig struct {
	Name     string `json:"Name" yaml:"Name" mapstructure:"Name"`
	Host     string `json:"Host" yaml:"Host" mapstructure:"Host"`
	Port     int    `json:"Port" yaml:"Port" mapstructure:"Port"`
	User     string `json:"User" yaml:"User" mapstructure:"User"`
	Password string `json:"Password" yaml:"Password" mapstructure:"Password"`
}

// 任务执行器配置
type ExecutorConfig struct {
	Script         string `json:"Script" yaml:"Script" mapstructure:"Script"`
	UploadScript   string `json:"UploadScript" yaml:"UploadScript" mapstructure:"UploadScript"`       // 文件上传脚本路径
	Concurrency    int    `json:"Concurrency" yaml:"Concurrency" mapstructure:"Concurrency"`          // 并发数
	TimeoutSeconds int    `json:"TimeoutSeconds" yaml:"TimeoutSeconds" mapstructure:"TimeoutSeconds"` // 超时时间
}

// Celery 配置
type CeleryConfig struct {
	Broker         string `json:"Broker" yaml:"Broker" mapstructure:"Broker"`
	Backend        string `json:"Backend" yaml:"Backend" mapstructure:"Backend"`
	TaskName       string `json:"TaskName" yaml:"TaskName" mapstructure:"TaskName"`
	UploadTaskName string `json:"UploadTaskName" yaml:"UploadTaskName" mapstructure:"UploadTaskName"` // 文件上传任务名称
	Workers        int    `json:"Workers" yaml:"Workers" mapstructure:"Workers"`
}
