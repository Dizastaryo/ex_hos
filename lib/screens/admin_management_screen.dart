import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/admin_service.dart';
import '../model/user_dto.dart';

class UserManagementScreen extends StatefulWidget {
  const UserManagementScreen({Key? key}) : super(key: key);

  @override
  State<UserManagementScreen> createState() => _UserManagementScreenState();
}

class _UserManagementScreenState extends State<UserManagementScreen> {
  late UserService userService;
  final TextEditingController _usernameController = TextEditingController();
  final TextEditingController _emailController = TextEditingController();
  final TextEditingController _passwordController = TextEditingController();
  final TextEditingController _searchController = TextEditingController();

  List<UserDTO> foundUsers = [];

  final Color primaryColor = const Color(0xFF30D5C8);

  @override
  void initState() {
    super.initState();
    userService = Provider.of<UserService>(context, listen: false);
  }

  Future<void> _createModerator() async {
    try {
      final response = await userService.createModerator(
        username: _usernameController.text,
        email: _emailController.text,
        password: _passwordController.text,
      );
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(response)),
      );
      _usernameController.clear();
      _emailController.clear();
      _passwordController.clear();
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка при создании модератора: $e')),
      );
    }
  }

  Future<void> _searchUsers() async {
    try {
      final users = await userService.searchUsers(_searchController.text);
      setState(() {
        foundUsers = users;
      });
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка при поиске пользователей: $e')),
      );
    }
  }

  Future<void> _blockUser(int userId) async {
    try {
      final response = await userService.blockUser(userId);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(response)),
      );
      _searchUsers();
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка при блокировке пользователя: $e')),
      );
    }
  }

  Future<void> _unblockUser(int userId) async {
    try {
      final response = await userService.unblockUser(userId);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(response)),
      );
      _searchUsers();
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка при разблокировке пользователя: $e')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Панель администратора'),
        backgroundColor: primaryColor,
        automaticallyImplyLeading: false,
        foregroundColor: Colors.white,
        elevation: 2,
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            _buildCard(
              title: 'Создать аккаунт для доктора',
              child: Column(
                children: [
                  _buildTextField(_usernameController, 'Имя пользователя'),
                  const SizedBox(height: 10),
                  _buildTextField(_emailController, 'Email'),
                  const SizedBox(height: 10),
                  _buildTextField(_passwordController, 'Пароль', obscure: true),
                  const SizedBox(height: 20),
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton.icon(
                      icon: const Icon(Icons.person_add),
                      label: const Text('Создать модератора'),
                      onPressed: _createModerator,
                      style: ElevatedButton.styleFrom(
                        backgroundColor: primaryColor,
                        padding: const EdgeInsets.symmetric(vertical: 14),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 24),
            _buildCard(
              title: 'Поиск пользователей',
              child: Column(
                children: [
                  _buildTextField(
                    _searchController,
                    'Введите имя пользователя',
                    suffix: IconButton(
                      icon: const Icon(Icons.search),
                      onPressed: _searchUsers,
                    ),
                  ),
                  const SizedBox(height: 20),
                  if (foundUsers.isEmpty) const Text('Нет результатов'),
                  ListView.builder(
                    shrinkWrap: true,
                    physics: const NeverScrollableScrollPhysics(),
                    itemCount: foundUsers.length,
                    itemBuilder: (context, index) {
                      final user = foundUsers[index];
                      return Card(
                        margin: const EdgeInsets.symmetric(vertical: 6),
                        elevation: 1,
                        child: ListTile(
                          leading: CircleAvatar(
                            backgroundColor: primaryColor,
                            child:
                                const Icon(Icons.person, color: Colors.white),
                          ),
                          title: Text(user.username),
                          subtitle: Text('ID: ${user.id}'),
                          trailing: Wrap(
                            spacing: 8,
                            children: [
                              IconButton(
                                icon: Icon(Icons.block,
                                    color: Colors.red.shade400),
                                tooltip: 'Заблокировать',
                                onPressed: () => _blockUser(user.id),
                              ),
                              IconButton(
                                icon:
                                    Icon(Icons.lock_open, color: primaryColor),
                                tooltip: 'Разблокировать',
                                onPressed: () => _unblockUser(user.id),
                              ),
                            ],
                          ),
                        ),
                      );
                    },
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCard({required String title, required Widget child}) {
    return Card(
      elevation: 3,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title,
                style:
                    const TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
            const SizedBox(height: 16),
            child,
          ],
        ),
      ),
    );
  }

  Widget _buildTextField(
    TextEditingController controller,
    String label, {
    bool obscure = false,
    Widget? suffix,
  }) {
    return TextField(
      controller: controller,
      obscureText: obscure,
      decoration: InputDecoration(
        labelText: label,
        suffixIcon: suffix,
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
        focusedBorder: OutlineInputBorder(
          borderSide: BorderSide(color: primaryColor, width: 2),
          borderRadius: BorderRadius.circular(12),
        ),
      ),
    );
  }
}
