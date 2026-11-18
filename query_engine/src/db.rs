use std::sync::Arc;

use storage_engine::{Catalog, Config, FsBackend};

use crate::{
    execution::scheduler::Scheduler,
    storage::{CatalogStorage, DataStorage, WrappedCatalog, WrappedStorage},
};

pub struct Database {
    pub scheduler: Arc<Scheduler>,
    pub engine: Arc<StorageEngine>,
}

pub struct StorageEngine {
    pub catalog: Arc<dyn CatalogStorage>,
    pub storage: Arc<dyn DataStorage>,
}

impl Database {
    pub fn new(conf: Option<Config>) -> Self {
        // Default to use an embedded storage engine instead of
        // a standalone storage engine
        let scheduler = Scheduler::new();
        let be = Arc::new(FsBackend::try_new(conf).unwrap());
        let engine = StorageEngine {
            catalog: Arc::new(WrappedCatalog {
                catalog: Arc::new(Catalog::new(be.clone())),
            }),
            storage: Arc::new(WrappedStorage {
                storage: Arc::new(storage_engine::Data::try_new(be).unwrap()),
            }),
        };
        Self {
            scheduler: Arc::new(scheduler),
            engine: Arc::new(engine),
        }
    }
}
