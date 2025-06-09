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
        backgroundColor: const Color(0xFF30D5C8), // Основной цвет для AppBar
        elevation: 2, // Легкая тень для современности
        automaticallyImplyLeading: false,

        titleTextStyle: const TextStyle(
          fontSize: 22,
          fontWeight: FontWeight.bold,
          color: Colors.white,
          fontFamily: 'Roboto', // Современный шрифт
        ),
      ),
      body: isLoading
          ? const Center(child: CircularProgressIndicator())
          : ListView.builder(
              padding: const EdgeInsets.all(12), // Отступы для списка
              itemCount: rooms.length,
              itemBuilder: (context, index) {
                final room = rooms[index];
                final roomNumber = room['room_number'];
                final username = room['username'];

                return Card(
                  margin: const EdgeInsets.symmetric(vertical: 8),
                  elevation: 4, // Тень для карточки
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12), // Скругленные углы
                  ),
                  child: Padding(
                    padding: const EdgeInsets.all(16), // Увеличенные отступы
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Кабинет №$roomNumber',
                          style: const TextStyle(
                            fontSize: 20,
                            fontWeight: FontWeight.bold,
                            color: Color(0xFF30D5C8), // Основной цвет
                            fontFamily: 'Roboto',
                          ),
                        ),
                        const SizedBox(height: 8),
                        Text(
                          'Специализация: ${room['specialization'] ?? '—'}',
                          style: const TextStyle(
                            fontSize: 16,
                            fontFamily: 'Roboto',
                          ),
                        ),
                        Text(
                          'Рабочие дни: ${room['work_days'] ?? '—'}',
                          style: const TextStyle(
                            fontSize: 16,
                            fontFamily: 'Roboto',
                          ),
                        ),
                        Text(
                          'Работа: ${room['start_time']}–${room['end_time']}',
                          style: const TextStyle(
                            fontSize: 16,
                            fontFamily: 'Roboto',
                          ),
                        ),
                        if (room['lunch_start'] != null &&
                            room['lunch_end'] != null)
                          Text(
                            'Обед: ${room['lunch_start']}–${room['lunch_end']}',
                            style: const TextStyle(
                              fontSize: 16,
                              fontFamily: 'Roboto',
                            ),
                          ),
                        const SizedBox(height: 12),
                        username != null
                            ? Row(
                                mainAxisAlignment:
                                    MainAxisAlignment.spaceBetween,
                                children: [
                                  Text(
                                    'Назначен: $username',
                                    style: TextStyle(
                                      fontSize: 16,
                                      fontFamily: 'Roboto',
                                      color: Colors.grey[700],
                                    ),
                                  ),
                                  ElevatedButton(
                                    onPressed: () => _unassignDoctor(
                                        roomNumber, room['user_id']),
                                    style: ElevatedButton.styleFrom(
                                      backgroundColor: const Color(0xFF30D5C8),
                                      shape: RoundedRectangleBorder(
                                        borderRadius: BorderRadius.circular(8),
                                      ),
                                      padding: const EdgeInsets.symmetric(
                                          horizontal: 16, vertical: 8),
                                    ),
                                    child: const Text(
                                      'Снять врача',
                                      style: TextStyle(
                                        fontFamily: 'Roboto',
                                        color: Colors.white,
                                      ),
                                    ),
                                  ),
                                ],
                              )
                            : Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  const Text(
                                    'Врач не назначен',
                                    style: TextStyle(
                                      fontSize: 16,
                                      fontFamily: 'Roboto',
                                      color: Colors.redAccent,
                                    ),
                                  ),
                                  DropdownButton<int>(
                                    hint: const Text(
                                      'Выберите врача',
                                      style: TextStyle(fontFamily: 'Roboto'),
                                    ),
                                    items: doctors.map((doctor) {
                                      return DropdownMenuItem<int>(
                                        value: doctor.id,
                                        child: Text(
                                          doctor.username,
                                          style: const TextStyle(
                                            fontFamily: 'Roboto',
                                            fontSize: 16,
                                          ),
                                        ),
                                      );
                                    }).toList(),
                                    onChanged: (doctorId) {
                                      if (doctorId != null) {
                                        _assignDoctor(roomNumber, doctorId);
                                      }
                                    },
                                    style: const TextStyle(
                                      fontSize: 16,
                                      color: Colors.black,
                                      fontFamily: 'Roboto',
                                    ),
                                    dropdownColor: Colors.white,
                                    icon: const Icon(
                                      Icons.arrow_drop_down,
                                      color: Color(0xFF30D5C8), // Основной цвет
                                    ),
                                    underline: Container(
                                      height: 2,
                                      color: const Color(0xFF30D5C8),
                                    ),
                                  ),
                                ],
                              ),
                      ],
                    ),
                  ),
                );
              },
            ),
    );
  }
}