package main

var cli struct {
	AccountID       string `help:"Aliyun account ID" env:"ALIBABA_CLOUD_ACCOUNT_ID"`
	AccessKeyID     string `help:"Aliyun access key ID" env:"ALIBABA_CLOUD_ACCESS_KEY_ID"`
	AccessKeySecret string `help:"Aliyun access key secret" env:"ALIBABA_CLOUD_ACCESS_KEY_SECRET"`

	DeployFC cmdDeployFC `cmd:"" help:"Deploy fc"`
	Build      cmdBuild      `cmd:"" help:"Build"`
	BuildDocker cmdBuildDocker `cmd:"" help:"Build Docker image"`
	Release     cmdRelease     `cmd:"" help:"Build release binaries for multiple platforms"`
	InvokeFC cmdInvokeFC `cmd:"" help:"Invoke fc"`
	Format   cmdFormat   `cmd:"" help:"Format and lint code"`
}

type cmdDeployFC struct {
}

func (c *cmdDeployFC) Run() error {
	deploy()
	return nil
}

type cmdBuild struct {
}

func (c *cmdBuild) Run() error {
	build()
	return nil
}

type cmdBuildDocker struct {
	Push bool `help:"Push image to registry after build" default:"false"`
}

func (c *cmdBuildDocker) Run() error {
	buildDocker()
	return nil
}

type cmdRelease struct {
}

func (c *cmdRelease) Run() error {
	release()
	return nil
}

type cmdInvokeFC struct {
	ServiceName  string `help:"FC service name" env:"FC_SERVICE_NAME"`
	FunctionName string `help:"FC function name" env:"FC_FUNCTION_NAME"`
	Payload      string `help:"Function invocation payload" default:"{}"`
}

func (c *cmdInvokeFC) Run() error {
	invokeFC()
	return nil
}

type cmdFormat struct {
}

func (c *cmdFormat) Run() error {
	format()
	return nil
}
