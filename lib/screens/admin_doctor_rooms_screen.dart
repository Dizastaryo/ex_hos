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

  final Color primaryColor = const Color(0xFF30D5C8);

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
        backgroundColor: primaryColor,
        automaticallyImplyLeading: false,
        foregroundColor: Colors.white,
        elevation: 2,
      ),
      body: isLoading
          ? const Center(child: CircularProgressIndicator())
          : ListView.builder(
              padding: const EdgeInsets.all(12),
              itemCount: rooms.length,
              itemBuilder: (context, index) {
                final room = rooms[index];
                final roomNumber = room['room_number'];
                final username = room['username'];

                return Card(
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(16)),
                  elevation: 3,
                  margin: const EdgeInsets.symmetric(vertical: 10),
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('Кабинет №$roomNumber',
                            style: const TextStyle(
                                fontSize: 18, fontWeight: FontWeight.bold)),
                        const SizedBox(height: 6),
                        Text('Специализация: ${room['specialization'] ?? '—'}'),
                        Text('Рабочие дни: ${room['work_days'] ?? '—'}'),
                        Text(
                            'Работа: ${room['start_time']}–${room['end_time']}'),
                        if (room['lunch_start'] != null &&
                            room['lunch_end'] != null)
                          Text(
                              'Обед: ${room['lunch_start']}–${room['lunch_end']}'),
                        const SizedBox(height: 12),
                        const Divider(),
                        const SizedBox(height: 8),
                        username != null
                            ? Row(
                                mainAxisAlignment:
                                    MainAxisAlignment.spaceBetween,
                                children: [
                                  Text('Назначен: $username',
                                      style: const TextStyle(
                                          fontWeight: FontWeight.w500)),
                                  OutlinedButton.icon(
                                    onPressed: () => _unassignDoctor(
                                        roomNumber, room['user_id']),
                                    icon: const Icon(Icons.person_remove),
                                    label: const Text('Снять врача'),
                                    style: OutlinedButton.styleFrom(
                                      foregroundColor: Colors.redAccent,
                                      side: const BorderSide(
                                          color: Colors.redAccent),
                                    ),
                                  ),
                                ],
                              )
                            : Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  const Text('Врач не назначен',
                                      style: TextStyle(
                                          fontWeight: FontWeight.w500)),
                                  const SizedBox(height: 8),
                                  DropdownButtonFormField<int>(
                                    decoration: InputDecoration(
                                      labelText: 'Выберите врача',
                                      border: OutlineInputBorder(
                                        borderRadius: BorderRadius.circular(12),
                                      ),
                                      focusedBorder: OutlineInputBorder(
                                        borderSide:
                                            BorderSide(color: primaryColor),
                                        borderRadius: BorderRadius.circular(12),
                                      ),
                                    ),
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
