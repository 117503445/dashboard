package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/117503445/goutils/gclient/aliyun"
	fc20230330 "github.com/alibabacloud-go/fc-20230330/v4/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/rs/zerolog/log"
)

func deployFunction(ctx context.Context, funcConfig FcFuncConfig) error {
	fcClient, err := aliyun.NewFc3Client(ctx, aliyun.Fc3ClientParams{
		Region:          "cn-hangzhou",
		AccountID:       cli.AccountID,
		AccessKeyId:     cli.AccessKeyID,
		AccessKeySecret: cli.AccessKeySecret,
	})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to get fc client")
		return err
	}

	log.Ctx(ctx).Info().Interface("funcConfig", funcConfig).Send()

	var codeZip string
	{
		codeZipBytes, err := os.ReadFile(funcConfig.FileCode)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("failed to read code zip")
			return err
		}
		codeZip = base64.StdEncoding.EncodeToString(codeZipBytes)
	}
	localHash := ""
	{
		codeZipBytes := []byte(codeZip)
		hash := sha256.Sum256(codeZipBytes)
		localHash = hex.EncodeToString(hash[:])
	}

	funcHash := ""
	{
		getResp, err := fcClient.GetFunction(tea.String(funcConfig.Name), &fc20230330.GetFunctionRequest{})
		if err != nil {
			if !strings.Contains(err.Error(), "FunctionNotFound") {
				log.Ctx(ctx).Error().Err(err).Msg("failed to get function")
				return err
			}

			log.Ctx(ctx).Info().Msg("function not found, creating new function")
			resp, err := fcClient.CreateFunction(&fc20230330.CreateFunctionRequest{
				Body: &fc20230330.CreateFunctionInput{
					FunctionName: tea.String(funcConfig.Name),
					Description:  tea.String(funcConfig.Description),
					Runtime:      tea.String(funcConfig.Runtime),
					Handler:      tea.String("main"),
					Code: &fc20230330.InputCodeLocation{
						ZipFile: tea.String(codeZip),
					},
				},
			})
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("failed to create function")
				return err
			}
			log.Ctx(ctx).Info().Interface("resp", resp).Send()
		} else {
			funcHash = tea.StringValue(getResp.Body.EnvironmentVariables["HASH"])
		}
	}

	{
		log.Ctx(ctx).Info().Msg("updating function")

		environmentVariables := map[string]*string{
			"HASH": tea.String(localHash),
		}
		for key, value := range funcConfig.EnvVars {
			environmentVariables[key] = tea.String(value)
		}

		input := &fc20230330.UpdateFunctionRequest{
			Body: &fc20230330.UpdateFunctionInput{
				Description: tea.String(funcConfig.Description),
				Runtime:     tea.String(funcConfig.Runtime),
				Handler:     tea.String(funcConfig.Handler),
				Code: &fc20230330.InputCodeLocation{
					ZipFile: tea.String(codeZip),
				},
				EnvironmentVariables: environmentVariables,
				Timeout:              tea.Int32(300),
				LogConfig:            funcConfig.LogConfig,
				Role:                 tea.String(funcConfig.Role),
				VpcConfig:            funcConfig.VPCConfig,
				CustomRuntimeConfig:  funcConfig.CustomRuntimeConfig,
			}}
		if localHash != funcHash {
			log.Ctx(ctx).Info().
				Str("localHash", localHash).
				Str("funcHash", funcHash).
				Msg("local hash is different from function hash, updating code")
			input.Body.Code = &fc20230330.InputCodeLocation{
				ZipFile: tea.String(codeZip),
			}
		} else {
			log.Ctx(ctx).Info().
				Str("localHash", localHash).
				Msg("local hash is the same as function hash, skipping code update")
		}

		_, err := fcClient.UpdateFunction(
			tea.String(funcConfig.Name),
			input,
		)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("failed to update function")
			return err
		}
	}

	if funcConfig.TimeTriggerCron != "" {
		type TimeTriggerConfig struct {
			Cron   string `json:"CronExpression"`
			Enable bool   `json:"Enable"`
		}

		triggerConfig := TimeTriggerConfig{
			Cron:   funcConfig.TimeTriggerCron,
			Enable: true,
		}

		configPayload, err := json.Marshal(triggerConfig)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("failed to marshal trigger config")
			return err
		}

		// 获取名为 timeTrigger 的触发器
		triggerName := "timeTrigger"
		getTriggerResp, err := fcClient.GetTrigger(tea.String(funcConfig.Name), tea.String(triggerName))
		if err != nil {
			// 触发器不存在，创建新的时间触发器
			if strings.Contains(err.Error(), "TriggerNotFound") {
				log.Ctx(ctx).Info().
					Str("triggerName", triggerName).
					Str("cron", funcConfig.TimeTriggerCron).
					Msg("trigger not found, creating new time trigger")

				createTriggerResp, createErr := fcClient.CreateTrigger(tea.String(funcConfig.Name), &fc20230330.CreateTriggerRequest{
					Body: &fc20230330.CreateTriggerInput{
						TriggerName:   tea.String(triggerName),
						TriggerType:   tea.String("timer"),
						TriggerConfig: tea.String(string(configPayload)),
					},
				})
				if createErr != nil {
					log.Ctx(ctx).Error().Err(createErr).Msg("failed to create trigger")
					return createErr
				}
				log.Ctx(ctx).Info().
					Str("triggerName", triggerName).
					Interface("resp", createTriggerResp).
					Msg("successfully created time trigger")
			} else {
				log.Ctx(ctx).Error().Err(err).Msg("failed to get trigger")
				return err
			}
		} else {
			log.Ctx(ctx).Info().
				Str("triggerName", triggerName).
				Interface("getTriggerResp", getTriggerResp).
				Msg("trigger found")

			curTriggerConfig := TimeTriggerConfig{}
			err = json.Unmarshal([]byte(tea.StringValue(getTriggerResp.Body.TriggerConfig)), &curTriggerConfig)
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("failed to unmarshal trigger config")
				return err
			}

			// 触发器存在，检查cron表达式是否一致
			if curTriggerConfig.Cron != funcConfig.TimeTriggerCron || !curTriggerConfig.Enable {
				log.Ctx(ctx).Info().
					Str("triggerName", triggerName).
					Str("currentCron", curTriggerConfig.Cron).
					Str("newCron", funcConfig.TimeTriggerCron).
					Bool("enable", curTriggerConfig.Enable).
					Msg("trigger config changed, updating trigger")

				updateTriggerResp, updateErr := fcClient.UpdateTrigger(tea.String(funcConfig.Name), tea.String(triggerName), &fc20230330.UpdateTriggerRequest{
					Body: &fc20230330.UpdateTriggerInput{
						TriggerConfig: tea.String(string(configPayload)),
					},
				})
				if updateErr != nil {
					log.Ctx(ctx).Error().Err(updateErr).Msg("failed to update trigger")
					return updateErr
				}
				log.Ctx(ctx).Info().
					Str("triggerName", triggerName).
					Interface("resp", updateTriggerResp).
					Msg("successfully updated time trigger")
			} else {
				log.Ctx(ctx).Info().
					Str("triggerName", triggerName).
					Str("cron", curTriggerConfig.Cron).
					Msg("cron expression is the same, skipping trigger update")
			}
		}
	}

	{
		resp, err := fcClient.GetAsyncInvokeConfig(tea.String(funcConfig.Name), &fc20230330.GetAsyncInvokeConfigRequest{})
		if err != nil || !tea.BoolValue(resp.Body.AsyncTask) {
			_, err := fcClient.PutAsyncInvokeConfig(tea.String(funcConfig.Name), &fc20230330.PutAsyncInvokeConfigRequest{
				Body: &fc20230330.PutAsyncInvokeConfigInput{
					AsyncTask: tea.Bool(true),
				},
			})
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("failed to put async invoke config")
				return err
			}
			log.Ctx(ctx).Info().Interface("resp", resp).Msg("async invoke config updated")
		}
		log.Ctx(ctx).Info().Interface("resp", resp).Msg("function retrieved")
	}

	return nil
}

type FcFuncConfig struct {
	Name            string
	Description     string
	FileCode        string
	EnvVars         map[string]string
	Handler         string
	TimeTriggerCron string
	Role            string
	VPCConfig       *fc20230330.VPCConfig
	LogConfig       *fc20230330.LogConfig

	Runtime             string
	CustomRuntimeConfig *fc20230330.CustomRuntimeConfig
}

func GetFuncConfigs(env string) []FcFuncConfig {
	return []FcFuncConfig{
		{
			Name:        "fc-event",
			Description: "fc-event",
			FileCode:    filepath.Join(dirProjectRoot, "data", "fc-event", "fc-event.zip"),
			EnvVars:     map[string]string{},
			Handler:     "fc-event",
			// TimeTriggerCron: "0 * * * * *", // 每分钟第 0 秒执行
			Role:    "acs:ram::" + cli.AccountID + ":role/aliyunfcdefaultrole",
			Runtime: "go1",
			// LogConfig: &fc20230330.LogConfig{
			// 	EnableInstanceMetrics: tea.Bool(true),
			// 	EnableRequestMetrics:  tea.Bool(true),
			// 	LogBeginRule:          tea.String("DefaultRegex"),
			// 	Logstore:              tea.String("default-logs"),
			// 	Project:               tea.String("serverless-cn-hangzhou-348eb21a-0e33-5ae1-98cd-eab208ff4cb6"),
			// },
		},
		{
			Name:        "fc-web",
			Description: "fc-web",
			FileCode:    filepath.Join(dirProjectRoot, "data", "fc-web", "fc-web.zip"),
			EnvVars:     map[string]string{},
			Handler:     "fc-web",
			Role:        "acs:ram::" + cli.AccountID + ":role/aliyunfcdefaultrole",
			Runtime:     "custom.debian10",
			CustomRuntimeConfig: &fc20230330.CustomRuntimeConfig{
				Port:    tea.Int32(8080),
				Command: tea.StringSlice([]string{"/code/fc-web"}),
			},
		},
	}
}

func deploy() {
	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	log.Ctx(ctx).Info().Msg("deploy")

	funcConfigs := GetFuncConfigs("")
	for _, funcConfig := range funcConfigs {
		err := deployFunction(ctx, funcConfig)
		if err != nil {
			log.Ctx(ctx).Panic().Err(err).Msg("failed to deploy function")
		}
	}
}
