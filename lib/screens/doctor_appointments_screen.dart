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
      appBar: AppBar(title: const Text("Мои пациенты")),
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
            return const Center(child: Text("Записей нет."));
          }

          return ListView.builder(
            itemCount: appointments.length,
            itemBuilder: (context, index) {
              final item = appointments[index];
              final appointment = item.appointment;

              return Card(
                margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                child: ListTile(
                  title: Text("Пациент: ${item.patientName}"),
                  subtitle: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const SizedBox(height: 4),
                      Text(
                          "Дата и время: ${_formatDateTime(appointment.appointmentTime)}"),
                      Text("Кабинет №${appointment.roomNumber}"),
                    ],
                  ),
                  leading: const Icon(Icons.event_available),
                  trailing: IconButton(
                    icon: const Icon(Icons.chat),
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
