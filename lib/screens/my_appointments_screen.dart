import 'package:flutter/material.dart';
import '../services/appointment_service.dart';

class MyAppointmentsPage extends StatefulWidget {
  final AppointmentService service;

  const MyAppointmentsPage({super.key, required this.service});

  @override
  State<MyAppointmentsPage> createState() => _MyAppointmentsPageState();
}

class _MyAppointmentsPageState extends State<MyAppointmentsPage> {
  late Future<List<dynamic>> _appointmentsFuture;

  @override
  void initState() {
    super.initState();
    _loadAppointments();
  }

  void _loadAppointments() {
    _appointmentsFuture = widget.service.getMyAppointments();
  }

  Future<void> _cancelAppointment(int appointmentId) async {
    try {
      await widget.service.cancelAppointment(appointmentId);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Запись успешно отменена'),
          backgroundColor: Colors.green,
        ),
      );
      setState(() {
        _loadAppointments(); // Обновляем список
      });
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Ошибка отмены: $e'),
          backgroundColor: Colors.red,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Мои записи')),
      body: FutureBuilder<List<dynamic>>(
        future: _appointmentsFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }

          if (snapshot.hasError) {
            return Center(child: Text('Ошибка: ${snapshot.error}'));
          }

          final appointments = snapshot.data ?? [];

          if (appointments.isEmpty) {
            return const Center(child: Text('Пока нету записей'));
          }

          return ListView.builder(
            padding: const EdgeInsets.all(16),
            itemCount: appointments.length,
            itemBuilder: (context, index) {
              final appointment = appointments[index];
              final appointmentTime =
                  DateTime.parse(appointment['appointment_time']);
              final formattedTime =
                  "${appointmentTime.toLocal().toString().replaceFirst('T', ' ').substring(0, 16)}";

              return Card(
                margin: const EdgeInsets.only(bottom: 12),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(12),
                ),
                child: ListTile(
                  title: Text(
                      'Кабинет №${appointment['room_number']} — ${appointment['specialization']}'),
                  subtitle: Text('Время: $formattedTime'),
                  trailing: IconButton(
                    icon: const Icon(Icons.cancel, color: Colors.red),
                    tooltip: 'Отменить запись',
                    onPressed: () {
                      showDialog(
                        context: context,
                        builder: (context) => AlertDialog(
                          title: const Text('Отмена записи'),
                          content: const Text(
                              'Вы уверены, что хотите отменить запись?'),
                          actions: [
                            TextButton(
                              onPressed: () => Navigator.of(context).pop(),
                              child: const Text('Нет'),
                            ),
                            TextButton(
                              onPressed: () {
                                Navigator.of(context).pop();
                                _cancelAppointment(appointment['id']);
                              },
                              child: const Text('Да'),
                            ),
                          ],
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
