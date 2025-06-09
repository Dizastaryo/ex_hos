import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../model/appointment.dart';
import '../services/appointment_service.dart';
import '../services/admin_service.dart';
import '../services/chat_service.dart';
import 'chat_moderator_page.dart';

class DoctorAppointmentsPage extends StatefulWidget {
  final AppointmentService appointmentService;
  final UserService userService;
  final ChatService chatService;

  const DoctorAppointmentsPage({
    Key? key,
    required this.appointmentService,
    required this.userService,
    required this.chatService,
  }) : super(key: key);

  @override
  State<DoctorAppointmentsPage> createState() => _DoctorAppointmentsPageState();
}

class _DoctorAppointmentsPageState extends State<DoctorAppointmentsPage> {
  late Future<List<_AppointmentWithPatientName>> _appointmentsWithNamesFuture;

  final Color primaryColor = const Color(0xFF30D5C8);

  @override
  void initState() {
    super.initState();
    _appointmentsWithNamesFuture = _loadAppointmentsWithUsernames();
  }

  Future<List<_AppointmentWithPatientName>>
      _loadAppointmentsWithUsernames() async {
    final appointments =
        await widget.appointmentService.getDoctorAppointments();

    final results = <_AppointmentWithPatientName>[];

    for (final appointment in appointments) {
      String patientName;
      try {
        patientName =
            await widget.userService.getUsernameById(appointment.patientId);
      } catch (_) {
        patientName = 'Неизвестно';
      }

      results.add(
        _AppointmentWithPatientName(
          appointment: appointment,
          patientName: patientName,
        ),
      );
    }

    return results;
  }

  String _formatDateTime(DateTime datetime) {
    return DateFormat('dd.MM.yyyy – HH:mm').format(datetime);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text("Мои пациенты"),
        backgroundColor: primaryColor,
        automaticallyImplyLeading: false,
        foregroundColor: Colors.white,
        elevation: 2,
      ),
      body: FutureBuilder<List<_AppointmentWithPatientName>>(
        future: _appointmentsWithNamesFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(child: Text("Ошибка: ${snapshot.error}"));
          }

          final appointments = snapshot.data!;
          if (appointments.isEmpty) {
            return const Center(
              child: Text(
                "Записей нет.",
                style: TextStyle(fontSize: 16, color: Colors.grey),
              ),
            );
          }

          return ListView.builder(
            padding: const EdgeInsets.all(16),
            itemCount: appointments.length,
            itemBuilder: (context, index) {
              final item = appointments[index];
              final appointment = item.appointment;

              return Card(
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(16),
                ),
                elevation: 3,
                margin: const EdgeInsets.only(bottom: 14),
                child: Padding(
                  padding:
                      const EdgeInsets.symmetric(vertical: 16, horizontal: 20),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          const Icon(Icons.person, color: Colors.black54),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(
                              item.patientName,
                              style: const TextStyle(
                                  fontSize: 18, fontWeight: FontWeight.w600),
                            ),
                          ),
                          IconButton(
                            icon: Icon(Icons.chat, color: primaryColor),
                            tooltip: 'Открыть чат',
                            onPressed: () {
                              Navigator.push(
                                context,
                                MaterialPageRoute(
                                  builder: (_) => ModeratorChatPage(
                                    userId: appointment.patientId,
                                    patientName: item.patientName,
                                    chatService: widget.chatService,
                                  ),
                                ),
                              );
                            },
                          ),
                        ],
                      ),
                      const SizedBox(height: 10),
                      Text(
                        "Дата и время: ${_formatDateTime(appointment.appointmentTime)}",
                        style: const TextStyle(fontSize: 14),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        "Кабинет №${appointment.roomNumber}",
                        style: const TextStyle(
                            fontSize: 14, color: Colors.black54),
                      ),
                    ],
                  ),
                ),
              );
            },
          );
        },
      ),
    );
  }
}

class _AppointmentWithPatientName {
  final Appointment appointment;
  final String patientName;

  _AppointmentWithPatientName({
    required this.appointment,
    required this.patientName,
  });
}
