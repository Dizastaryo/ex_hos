// lib/models/appointment.dart

class Appointment {
  final DateTime appointmentTime;
  final int patientId;
  final int roomNumber;

  Appointment({
    required this.appointmentTime,
    required this.patientId,
    required this.roomNumber,
  });

  /// Создаёт объект из JSON
  factory Appointment.fromJson(Map<String, dynamic> json) {
    return Appointment(
      appointmentTime: DateTime.parse(json['appointment_time'] as String),
      patientId: json['patient_id'] as int,
      roomNumber: json['room_number'] as int,
    );
  }

  /// Преобразует объект в JSON
  Map<String, dynamic> toJson() {
    return {
      'appointment_time': appointmentTime.toIso8601String(),
      'patient_id': patientId,
      'room_number': roomNumber,
    };
  }
}
