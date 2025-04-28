# TODOs for `localnet`

**Basics**
- [x] Rename the chains to clients (or something else that is a bit clearer)
- [x] Remove the git submodule
- [x] Spin up Ethereum chain as well in e2e test
- [ ] 

**Cleanup**
- [ ] Unify `Spinup `to do the same things:
    - [x] Start the chain
    - [ ] Return a chain client with wallets added
- [ ] Clean up all unused stuff
- [ ] Figure out design that makes sense for `chainclients` that doesn't have ibc
- [ ] Make a better separation between chain and node
- [ ] Use cosmos.Wallet as the wallet in `cosmoschain`
- [ ] Use chain/client for internals in `cosmoschain`
- [ ] Design relaying

**Later**
- [ ] Maybe find a better name for `cosmoschain`?
- [ ] Change to use eureka-ops for deployment? Or just write my own deployment scripts?

