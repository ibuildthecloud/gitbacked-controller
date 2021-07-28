Example
=======

This is dumb controller that will ensure that X number of `Replicated` objects will exist for each
`Replicator` object.  To run this do as follows:


1. Create new repo somewhere (github for example)
2. Compile and run this controller passing your new repo URL, not the example one
    ```shell
    go run main.go -url git@github.com:ibuildthecloud/test.git
    ```
3. Create a new resource in the repo
    ```yaml
    kind: Replicator
    apiVersion: example.com/v1
    metadata:
      name: asdf
    count: 5
    selector:
      foo: bar
    ```
    ```shell
    git add example.yaml
    git commit -a -m test
    git push
    ```
4. After about 15 seconds the controller should get an event that a `Replicator` was added
   and then create 5 `Replicated` objects and push those to git.  If you delete one of the
   newly created files, it will be detected and recreated.  The code is dumb and will not
   "scale down" and also the git code doesn't support garbage collection, so just don't expect
   those to work
