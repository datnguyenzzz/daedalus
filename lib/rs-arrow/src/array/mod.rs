pub mod boolean_array;
pub mod primitive_array;
pub mod select;
pub mod string_array;
use std::{any::Any, sync::Arc};

pub use boolean_array::BooleanArray;
pub use primitive_array::PrimitiveArray;
pub use string_array::StringArray;

use crate::{ArrayRef, DataType};

/// Trait implemented by native Rust primitive types that can be stored in `PrimitiveArray`
pub trait NativeType: Send + Sync + Copy + 'static {
    fn data_type() -> DataType;
    fn default() -> Self;
    fn as_any(&self) -> &dyn Any {
        self
    }
}

macro_rules! define_native_types {
    ($(
        ($ty:ty, $dt:path, $default:expr)
    ),* $(,)?) => {
        $(
            impl NativeType for $ty {
                #[inline]
                fn data_type() -> DataType {
                    $dt
                }
                #[inline]
                fn default() -> Self {
                    $default
                }
            }
        )*
    };
}

/// Matches a runtime `DataType` enum variant and dispatches a generic closure/block
/// bound to the correct, physical concrete Rust type.
#[macro_export]
macro_rules! dispatch_native_type {
    ($data_type:expr, $type_var:ident, $body:block) => {
        match $data_type {
            $crate::DataType::Int8 => {
                type $type_var = i8;
                $body
            }
            $crate::DataType::Int16 => {
                type $type_var = i16;
                $body
            }
            $crate::DataType::Int32 => {
                type $type_var = i32;
                $body
            }
            $crate::DataType::Int64 => {
                type $type_var = i64;
                $body
            }
            $crate::DataType::Float32 => {
                type $type_var = f32;
                $body
            }
            $crate::DataType::Float64 => {
                type $type_var = f64;
                $body
            }
            $crate::DataType::Boolean => {
                type $type_var = bool;
                $body
            }
            other => panic!("DataType {:?} is not a native primitive type", other),
        }
    };
}

define_native_types! {
    (bool, DataType::Boolean, false),
    (i8, DataType::Int8, 0i8),
    (i16, DataType::Int16, 0i16),
    (i32, DataType::Int32, 0i32),
    (i64, DataType::Int64, 0i64),
    (f32, DataType::Float32, 0.0f32),
    (f64, DataType::Float64, 0.0f64),
}

pub fn new_empty_array(dt: DataType) -> ArrayRef {
    match dt {
        DataType::Boolean => Arc::new(BooleanArray::from(Vec::<bool>::with_capacity(0))),
        DataType::Int8 => Arc::new(PrimitiveArray::<i8>::from(Vec::<i8>::with_capacity(0))),
        DataType::Int16 => Arc::new(PrimitiveArray::<i16>::from(Vec::<i16>::with_capacity(0))),
        DataType::Int32 => Arc::new(PrimitiveArray::<i32>::from(Vec::<i32>::with_capacity(0))),
        DataType::Int64 => Arc::new(PrimitiveArray::<i64>::from(Vec::<i64>::with_capacity(0))),
        DataType::Float32 => Arc::new(PrimitiveArray::<f32>::from(Vec::<f32>::with_capacity(0))),
        DataType::Float64 => Arc::new(PrimitiveArray::<f64>::from(Vec::<f64>::with_capacity(0))),
        DataType::Utf8 => Arc::new(StringArray::from(Vec::<String>::with_capacity(0))),
    }
}
