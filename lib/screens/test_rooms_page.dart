// test_rooms_page.dart
import 'package:flutter/material.dart';
import 'package:table_calendar/table_calendar.dart';
import '../services/appointment_service.dart';
import 'slots_page.dart';

class TestRoomsPage extends StatefulWidget {
  final AppointmentService service;
  const TestRoomsPage({super.key, required this.service});

  @override
  State<TestRoomsPage> createState() => _TestRoomsPageState();
}

class _TestRoomsPageState extends State<TestRoomsPage> {
  late Future<List<dynamic>> _rooms;

  @override
  void initState() {
    super.initState();
    _rooms = widget.service.getTestRooms();
  }

  void _selectDateAndNavigate(int roomNumber) async {
    final today = DateTime.now();
    final selectedDate = await showDatePicker(
      context: context,
      initialDate: today,
      firstDate: today,
      lastDate: today.add(const Duration(days: 30)),
      builder: (context, child) => Theme(
        data: ThemeData.light().copyWith(
          colorScheme: ColorScheme.light(
            primary: Colors.deepOrange,
            onPrimary: Colors.white,
            surface: Colors.white,
            onSurface: Colors.black,
          ),
          dialogBackgroundColor: Colors.white,
        ),
        child: child!,
      ),
    );

    if (selectedDate != null) {
      Navigator.push(
        context,
        MaterialPageRoute(
          builder: (_) => SlotsPage(
            service: widget.service,
            roomNumber: roomNumber,
            selectedDate: selectedDate,
          ),
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text("Кабинеты для анализов")),
      body: FutureBuilder<List<dynamic>>(
        future: _rooms,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(child: Text("Ошибка: \${snapshot.error}"));
          }

          final rooms = snapshot.data!;
          return ListView.builder(
            itemCount: rooms.length,
            itemBuilder: (context, index) {
              final room = rooms[index];
              return ListTile(
                title: Text(
                    "Кабинет №\${room['room_number']} — \${room['specialization']}"),
                subtitle: Text(
                    "Время: \${room['start_time']} - \${room['end_time']}"),
                trailing: const Icon(Icons.calendar_today),
                onTap: () => _selectDateAndNavigate(room['room_number']),
              );
            },
          );
        },
      ),
    );
  }
}
