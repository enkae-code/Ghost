fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Compile the proto file from the project root
    tonic_build::compile_protos("../ghost.proto")?;
    Ok(())
}
