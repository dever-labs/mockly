use std::net::TcpListener;

/// Bind to port 0, get the assigned port, immediately drop the listener.
/// Call sequentially to minimize TOCTOU window.
pub fn get_free_port() -> std::io::Result<u16> {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    Ok(listener.local_addr()?.port())
}

pub fn is_port_conflict(msg: &str) -> bool {
    let lower = msg.to_lowercase();
    lower.contains("address already in use")
        || lower.contains("eaddrinuse")
        || lower.contains("bind")
}
