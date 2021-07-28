# Git Backed Controller

The basic idea is to write a k8s controller that runs against git and not k8s apiserver. So the
controller is reading and writing objects to git and responding to changes in git. This is
accomplished by creating a custom implemenation of the clients and caches in controller-runtime.

## Status

This is totally hacked up and just proving and idea. There's most likely a lot of issues with this
code.  For one the RESTMapping logic is completely wrong as offline you have knowledge if a resource
is namespaced or not.

## Basic approach

You should just be able to write a controller as normal but in the setup just set the NewClient, NewCache, and MapperProvider to the git based ones.

```golang
	git, err := gitbacked.New(ctx, url, gitbacked.Options{
		Branch:       branch,
		SubDirectory: subdir,
		Interval:     interval,
	})
	defer git.Close()

	mgr, err := ctrl.NewManager(&rest.Config{}, ctrl.Options{
		Scheme:         scheme,
		NewClient:      git.NewClient,
		NewCache:       git.NewCache,
		MapperProvider: git.MapperProvider,
	})
```

## Authentication

The controller will pull from and push to the same branch.  Right now the code will just call `git push` so it is expect that that call will work with no use input (ssh keys or some agent based setup is in place).

## Example

A more complete example is in the `./example` folder.
