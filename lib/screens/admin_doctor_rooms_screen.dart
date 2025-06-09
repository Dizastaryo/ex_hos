import 'package:flutter/material.dart';
import '../services/admin_service.dart';
import '../services/doctor_room_service.dart';
import '../model/user_dto.dart';

class AdminDoctorRoomsPage extends StatefulWidget {
  final UserService userService;
  final DoctorRoomService doctorRoomService;

  const AdminDoctorRoomsPage({
    Key? key,
    required this.userService,
    required this.doctorRoomService,
  }) : super(key: key);

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
    setState(() => isLoading = true);
    try {
      final fetchedRooms = await widget.doctorRoomService.getDoctorRooms();
      final fetchedDoctors = await widget.userService.getAllDoctors();
      for (var room in fetchedRooms) {
        final userId = room['user_id'];
        if (userId != null) {
          room['username'] = await widget.userService.getUsernameById(userId);
        }
      }
      setState(() {
        rooms = fetchedRooms;
        doctors = fetchedDoctors;
      });
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка загрузки: \$e')),
      );
    } finally {
      setState(() => isLoading = false);
    }
  }

  Future<void> _assignDoctor(int roomNumber, int doctorId) async {
    try {
      await widget.doctorRoomService.assignDoctorToRoom(roomNumber, doctorId);
      await _loadData();
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка назначения: \$e')),
      );
    }
  }

  Future<void> _unassignDoctor(int roomNumber, int doctorId) async {
    try {
      await widget.doctorRoomService
          .unassignDoctorFromRoom(roomNumber, doctorId);
      await _loadData();
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка снятия: \$e')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        backgroundColor: const Color(0xFF30D5C8),
        title: const Text('Кабинеты и врачи'),
        elevation: 2,
        centerTitle: true,
      ),
      body: isLoading
          ? const Center(
              child: CircularProgressIndicator(color: Color(0xFF30D5C8)))
          : RefreshIndicator(
              color: const Color(0xFF30D5C8),
              onRefresh: _loadData,
              child: ListView.separated(
                padding: const EdgeInsets.all(16),
                itemCount: rooms.length,
                separatorBuilder: (_, __) => const SizedBox(height: 12),
                itemBuilder: (context, index) {
                  final room = rooms[index];
                  final roomNumber = room['room_number'];
                  final username = room['username'];
                  final specialization = room['specialization'] ?? '—';
                  final workDays = room['work_days'] ?? '—';
                  final startTime = room['start_time'];
                  final endTime = room['end_time'];
                  final lunchStart = room['lunch_start'];
                  final lunchEnd = room['lunch_end'];

                  return Card(
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(16),
                    ),
                    elevation: 4,
                    child: Padding(
                      padding: const EdgeInsets.all(16),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Icon(Icons.meeting_room,
                                  size: 28, color: Color(0xFF30D5C8)),
                              const SizedBox(width: 8),
                              Text('Кабинет №$roomNumber',
                                  style: TextStyle(
                                      fontSize: 20,
                                      fontWeight: FontWeight.bold,
                                      color: Colors.grey[800])),
                            ],
                          ),
                          const SizedBox(height: 8),
                          Wrap(
                            spacing: 16,
                            runSpacing: 8,
                            children: [
                              _InfoChip(label: 'Спец.: $specialization'),
                              _InfoChip(label: 'Дни: $workDays'),
                              _InfoChip(label: 'Время: \$startTime–\$endTime'),
                              if (lunchStart != null && lunchEnd != null)
                                _InfoChip(
                                    label: 'Обед: \$lunchStart–\$lunchEnd'),
                            ],
                          ),
                          const SizedBox(height: 12),
                          username != null
                              ? ListTile(
                                  contentPadding: EdgeInsets.zero,
                                  leading: Icon(Icons.person,
                                      color: Color(0xFF30D5C8)),
                                  title: Text('Назначен: \$username'),
                                  trailing: TextButton.icon(
                                    style: TextButton.styleFrom(
                                      foregroundColor: Color(0xFF30D5C8),
                                    ),
                                    onPressed: () => _unassignDoctor(
                                        roomNumber, room['user_id']),
                                    icon: Icon(Icons.remove_circle_outline),
                                    label: Text('Снять'),
                                  ),
                                )
                              : Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text('Врач не назначен',
                                        style: TextStyle(
                                            fontSize: 16,
                                            color: Colors.grey[700])),
                                    const SizedBox(height: 8),
                                    DropdownButtonFormField<int>(
                                      decoration: InputDecoration(
                                        border: OutlineInputBorder(
                                          borderRadius:
                                              BorderRadius.circular(8),
                                        ),
                                      ),
                                      hint: Text('Выберите врача'),
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
                                    ),
                                  ],
                                ),
                        ],
                      ),
                    ),
                  );
                },
              ),
            ),
    );
  }
}

class _InfoChip extends StatelessWidget {
  final String label;
  const _InfoChip({required this.label});

  @override
  Widget build(BuildContext context) {
    return Chip(
      backgroundColor: Colors.grey[200],
      label: Text(label, style: TextStyle(fontSize: 14)),
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
    );
  }
}
