import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../services/appointment_service.dart';
import '../services/admin_service.dart';
import 'chat_moderator_page.dart';

class DoctorAppointmentsPage extends StatefulWidget {
  final AppointmentService appointmentService;
  final UserService userService;

  const DoctorAppointmentsPage({
    Key? key,
    required this.appointmentService,
    required this.userService,
  }) : super(key: key);

  @override
  State<DoctorAppointmentsPage> createState() => _DoctorAppointmentsPageState();
}

class _DoctorAppointmentsPageState extends State<DoctorAppointmentsPage> {
  late Future<List<Map<String, dynamic>>> _appointmentsWithNamesFuture;

  @override
  void initState() {
    super.initState();
    _appointmentsWithNamesFuture = _loadAppointmentsWithUsernames();
  }

  Future<List<Map<String, dynamic>>> _loadAppointmentsWithUsernames() async {
    final appointments =
        await widget.appointmentService.getDoctorAppointments();
    final List<Map<String, dynamic>> results = [];

    for (final item in appointments) {
      try {
        final username =
            await widget.userService.getUsernameById(item['user_id']);
        results.add({
          ...item,
          'patient_name': username,
        });
      } catch (_) {
        results.add({
          ...item,
          'patient_name': 'Неизвестно',
        });
      }
    }

    return results;
  }

  String _formatDateTime(String datetime) {
    final dateTime = DateTime.parse(datetime);
    return DateFormat('dd.MM.yyyy – HH:mm').format(dateTime);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text("Мои пациенты")),
      body: FutureBuilder<List<Map<String, dynamic>>>(
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
              final userId = item['user_id'] as int;
              final patientName = item['patient_name'] as String;

              return Card(
                margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                child: ListTile(
                  title: Text("Пациент: $patientName"),
                  subtitle: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const SizedBox(height: 4),
                      Text(
                          "Дата и время: ${_formatDateTime(item['appointment_time'])}"),
                      Text("Кабинет №${item['room_number']}"),
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
                            userId: userId,
                            patientName: patientName,
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
