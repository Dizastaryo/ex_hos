import 'package:flutter/material.dart';
import '../services/admin_service.dart';
import '../services/doctor_room_service.dart';
import '../model/user_dto.dart';

class AdminDoctorRoomsPage extends StatefulWidget {
  final UserService userService;
  final DoctorRoomService doctorRoomService;

  const AdminDoctorRoomsPage({
    super.key,
    required this.userService,
    required this.doctorRoomService,
  });

  @override
  State<AdminDoctorRoomsPage> createState() => _AdminDoctorRoomsPageState();
}

class _AdminDoctorRoomsPageState extends State<AdminDoctorRoomsPage> {
  List<dynamic> rooms = [];
  List<UserDTO> doctors = [];
  bool isLoading = true;

  @override
  void initState() {
    super.initState();
    _loadData();
  }

  Future<void> _loadData() async {
    try {
      final fetchedRooms = await widget.doctorRoomService.getDoctorRooms();
      final fetchedDoctors = await widget.userService.getAllDoctors();

      for (var room in fetchedRooms) {
        final userId = room['user_id'];
        if (userId != null) {
          final username = await widget.userService.getUsernameById(userId);
          room['username'] = username;
        }
      }

      setState(() {
        rooms = fetchedRooms;
        doctors = fetchedDoctors;
        isLoading = false;
      });
    } catch (e) {
      setState(() => isLoading = false);
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('Ошибка загрузки: $e')));
    }
  }

  Future<void> _assignDoctor(int roomNumber, int doctorId) async {
    try {
      await widget.doctorRoomService.assignDoctorToRoom(roomNumber, doctorId);
      await _loadData();
    } catch (e) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('Ошибка назначения: $e')));
    }
  }

  Future<void> _unassignDoctor(int roomNumber, int doctorId) async {
    try {
      await widget.doctorRoomService
          .unassignDoctorFromRoom(roomNumber, doctorId);
      await _loadData();
    } catch (e) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('Ошибка снятия: $e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Кабинеты и врачи'),
        automaticallyImplyLeading: false,
      ),
      body: isLoading
          ? const Center(child: CircularProgressIndicator())
          : ListView.builder(
              itemCount: rooms.length,
              itemBuilder: (context, index) {
                final room = rooms[index];
                final roomNumber = room['room_number'];
                final username = room['username'];

                return Card(
                  margin: const EdgeInsets.all(8),
                  child: Padding(
                    padding: const EdgeInsets.all(12),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('Кабинет №$roomNumber',
                            style: const TextStyle(
                                fontSize: 18, fontWeight: FontWeight.bold)),
                        const SizedBox(height: 4),
                        Text('Специализация: ${room['specialization'] ?? '—'}'),
                        Text('Рабочие дни: ${room['work_days'] ?? '—'}'),
                        Text(
                            'Работа: ${room['start_time']}–${room['end_time']}'),
                        if (room['lunch_start'] != null &&
                            room['lunch_end'] != null)
                          Text(
                              'Обед: ${room['lunch_start']}–${room['lunch_end']}'),
                        const SizedBox(height: 8),
                        username != null
                            ? Row(
                                mainAxisAlignment:
                                    MainAxisAlignment.spaceBetween,
                                children: [
                                  Text('Назначен: $username'),
                                  ElevatedButton(
                                    onPressed: () => _unassignDoctor(
                                        roomNumber, room['user_id']),
                                    child: const Text('Снять врача'),
                                  )
                                ],
                              )
                            : Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  const Text('Врач не назначен'),
                                  DropdownButton<int>(
                                    hint: const Text('Выберите врача'),
                                    items: doctors.map((doctor) {
                                      return DropdownMenuItem<int>(
                                        value: doctor.id,
                                        child: Text(doctor.username),
                                      );
                                    }).toList(),
                                    onChanged: (doctorId) {
                                      if (doctorId != null) {
                                        _assignDoctor(roomNumber, doctorId);
                                      }
                                    },
                                  )
                                ],
                              )
                      ],
                    ),
                  ),
                );
              },
            ),
    );
  }
}
