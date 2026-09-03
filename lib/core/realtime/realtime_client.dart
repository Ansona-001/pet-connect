import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../network/api_client.dart';

final realtimeClientProvider = Provider<RealtimeClient>((ref) {
  final client = RealtimeClient(ref.watch(apiClientProvider));
  ref.onDispose(client.stop);
  return client;
});

class RealtimeEvent {
  const RealtimeEvent({required this.type, required this.data});

  final String type;
  final Map<String, dynamic> data;
}

class RealtimeClient {
  RealtimeClient(this._api);

  final ApiClient _api;
  final _events = StreamController<RealtimeEvent>.broadcast();
  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _socketSubscription;
  Timer? _reconnectTimer;
  bool _shouldRun = false;
  bool _connecting = false;
  int _attempt = 0;

  Stream<RealtimeEvent> get events => _events.stream;

  Future<void> start() async {
    _shouldRun = true;
    await _connect();
  }

  Future<void> _connect() async {
    if (!_shouldRun || _connecting || _channel != null) return;
    _connecting = true;
    try {
      final response = await _api.dio.post<Map<String, dynamic>>(
        '/realtime/tickets',
      );
      final ticket = response.data?['data']?['ticket']?.toString();
      if (ticket == null || ticket.isEmpty) {
        throw const FormatException('Missing realtime ticket.');
      }
      final apiUri = Uri.parse(_api.config.apiBaseUrl);
      final uri = apiUri.replace(
        scheme: apiUri.scheme == 'https' ? 'wss' : 'ws',
        path: '${apiUri.path}/realtime',
        queryParameters: {'ticket': ticket},
      );
      final channel = WebSocketChannel.connect(uri);
      await channel.ready.timeout(const Duration(seconds: 8));
      if (!_shouldRun) {
        await channel.sink.close();
        return;
      }
      _channel = channel;
      _attempt = 0;
      _socketSubscription = channel.stream.listen(
        _onMessage,
        onError: (_) => _onDisconnected(),
        onDone: _onDisconnected,
        cancelOnError: true,
      );
    } catch (_) {
      _scheduleReconnect();
    } finally {
      _connecting = false;
    }
  }

  void _onMessage(dynamic value) {
    try {
      final decoded = jsonDecode(value.toString());
      if (decoded is! Map) return;
      final event = Map<String, dynamic>.from(decoded);
      final type = event['type']?.toString();
      final data = event['data'];
      if (type == null || data is! Map) return;
      _events.add(
        RealtimeEvent(type: type, data: Map<String, dynamic>.from(data)),
      );
    } catch (_) {
      // Ignore malformed frames; durable REST cursors remain the source of truth.
    }
  }

  void _onDisconnected() {
    _socketSubscription = null;
    _channel = null;
    _scheduleReconnect();
  }

  void _scheduleReconnect() {
    if (!_shouldRun || _reconnectTimer != null) return;
    final seconds = 1 << _attempt.clamp(0, 5);
    _attempt++;
    _reconnectTimer = Timer(Duration(seconds: seconds), () {
      _reconnectTimer = null;
      unawaited(_connect());
    });
  }

  Future<void> stop() async {
    _shouldRun = false;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    await _socketSubscription?.cancel();
    _socketSubscription = null;
    await _channel?.sink.close();
    _channel = null;
  }
}
