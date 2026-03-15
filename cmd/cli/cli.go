package main

var cli struct {
	Test CmdTest `cmd:"" help:"Test command"`
}

type CmdTest struct {
}

func (cmd *CmdTest) Run() error {
	test()
	return nil
}
