# Operator

The operator itself accepts command line arguments to override some of the default behaviours. They are as follows.

| **Command line argument**      | **Type** | **Description**                                                                                            | **Default Value**          |   |
|--------------------------------|----------|------------------------------------------------------------------------------------------------------------|----------------------------|---|
| `--cluster-resource-namespace` | string   | The default namespace for cluster scoped resources                                                         | cloudflare-operator-system |   |
| `--overwrite-unmanaged-dns`    | boolean  | Overwrite existing DNS records that do not have a corresponding managed TXT record                         | false                      |   |
| `--leader-elect`               | boolean  | Enable leader election for controller manager, this is optional for operator running with a single replica | true                       |   |
