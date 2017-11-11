package cmd

import (
	"bytes"
	"fmt"

	"github.com/codegangsta/cli"
	"github.com/disorganizer/brig/brigd/client"
	yml "gopkg.in/yaml.v2"
)

func remoteListToYml(remotes []client.Remote) ([]byte, error) {
	// TODO: Provide a nicer representation here.
	return yml.Marshal(remotes)
}

func ymlToRemoteList(data []byte) ([]client.Remote, error) {
	remotes := []client.Remote{}
	if err := yml.Unmarshal(data, remotes); err != nil {
		return nil, err
	}

	return remotes, nil
}

func handleRemoteAdd(ctx *cli.Context, ctl *client.Client) error {
	remote := client.Remote{
		Fingerprint: "",
		Name:        "",
		Folders:     nil,
	}

	if err := ctl.RemoteAdd(remote); err != nil {
		return fmt.Errorf("remote add: %v", err)
	}

	return nil
}

func handleRemoteRemove(ctx *cli.Context, ctl *client.Client) error {
	name := ctx.Args().First()
	if err := ctl.RemoteRm(name); err != nil {
		return fmt.Errorf("remote rm: %v", err)
	}

	return nil
}

func handleRemoteList(ctx *cli.Context, ctl *client.Client) error {
	remotes, err := ctl.RemoteLs()
	if err != nil {
		return fmt.Errorf("remote ls: %v", err)
	}

	data, err := remoteListToYml(remotes)
	if err != nil {
		return fmt.Errorf("Failed to convert to yml: %v", err)
	}

	fmt.Println(data)
	return nil
}

func handleRemoteEdit(ctx *cli.Context, ctl *client.Client) error {
	remotes, err := ctl.RemoteLs()
	if err != nil {
		return fmt.Errorf("remote ls: %v", err)
	}

	data, err := remoteListToYml(remotes)
	if err != nil {
		return fmt.Errorf("Failed to convert to yml: %v", err)
	}

	newData, err := edit(data)
	if err != nil {
		return fmt.Errorf("Failed to launch editor: %v", err)
	}

	// Save a few network roundtrips if nothing was changed:
	if bytes.Equal(data, newData) {
		fmt.Println("Nothing changed.")
		return nil
	}

	newRemotes, err := ymlToRemoteList(newData)
	if err != nil {
		return err
	}

	if err := ctl.RemoteSave(newRemotes); err != nil {
		return fmt.Errorf("Saving back remotes failed: %v", err)
	}

	return nil
}

func handleRemoteLocate(ctx *cli.Context, ctl *client.Client) error {
	who := ctx.Args().First()
	candidates, err := ctl.RemoteLocate(who)
	if err != nil {
		return fmt.Errorf("Failed to locate peers: %v", err)
	}

	for _, candidate := range candidates {
		fmt.Println(candidate.Fingerprint)
	}

	return nil
}

func handleRemoteSelf(ctx *cli.Context, ctl *client.Client) error {
	// TODO: Implement backend
	return nil
}
