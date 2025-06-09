import 'package:flutter/material.dart';
import '../services/appointment_service.dart';
import 'slots_page.dart';

class DoctorRoomsPage extends StatefulWidget {
  final AppointmentService service;
  const DoctorRoomsPage({super.key, required this.service});

  @override
  State<DoctorRoomsPage> createState() => _DoctorRoomsPageState();
}

class _DoctorRoomsPageState extends State<DoctorRoomsPage> {
  late Future<List<dynamic>> _rooms;

  @override
  void initState() {
    super.initState();
    _rooms = widget.service.getDoctorRooms();
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
          colorScheme: const ColorScheme.light(
            primary: Color(0xFF30d5c8),
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

  Widget _buildRoomCard(Map<String, dynamic> room) {
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      elevation: 4,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: ListTile(
        contentPadding: const EdgeInsets.all(16),
        leading: const Icon(Icons.local_hospital,
            color: Color(0xFF30d5c8), size: 36),
        title: Text(
          "Кабинет №${room['room_number']}",
          style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 18),
        ),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const SizedBox(height: 4),
            Text("${room['specialization']}",
                style: const TextStyle(fontSize: 16)),
            const SizedBox(height: 4),
            Text("Время: ${room['start_time']} - ${room['end_time']}"),
            Text("Обед: ${room['lunch_start']} - ${room['lunch_end']}"),
          ],
        ),
        trailing: Icon(Icons.calendar_today, color: Colors.grey[600]),
        onTap: () => _selectDateAndNavigate(room['room_number']),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text("Кабинеты врачей"),
        backgroundColor: const Color(0xFF30d5c8),
        foregroundColor: Colors.white,
        elevation: 2,
      ),
      body: FutureBuilder<List<dynamic>>(
        future: _rooms,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(
                child: CircularProgressIndicator(color: Color(0xFF30d5c8)));
          }
          if (snapshot.hasError) {
            return Center(child: Text("Ошибка: ${snapshot.error}"));
          }

          final rooms = snapshot.data!;
          return ListView.builder(
            itemCount: rooms.length,
            itemBuilder: (context, index) => _buildRoomCard(rooms[index]),
          );
        },
      ),
    );
  }
}
