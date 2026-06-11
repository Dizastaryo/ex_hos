-- BLE device registry: каждая запись = один физический браслет
-- Генерируется в admin-панели, ID вшиваются в прошивку браслета
-- Юзер привязывает браслет по serial_number (с QR-наклейки)

CREATE TABLE ble_devices (
    id              UUID    PRIMARY KEY DEFAULT uuid_generate_v4(),
    serial_number   TEXT    UNIQUE NOT NULL,   -- SEEU_000001 (печатается на QR)
    public_id_hex   TEXT    UNIQUE NOT NULL,   -- 16-символьный hex (= mode=0x00 broadcast id)
    private_id_hex  TEXT    UNIQUE NOT NULL,   -- 16-символьный hex (= mode=0x01 broadcast id)
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    notes           TEXT    NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ble_devices_public  ON ble_devices(public_id_hex);
CREATE INDEX idx_ble_devices_private ON ble_devices(private_id_hex);
CREATE INDEX idx_ble_devices_serial  ON ble_devices(serial_number);
