const { createApp, ref, onMounted, computed, watch, nextTick } = Vue;

createApp({
    setup() {
        const view = ref('admin');
        const adminTab = ref('players');
        const players = ref([]);
        const flights = ref([]);
        const results = ref([]);
        const course = ref([]);
        const newFlightName = ref('');
        const flightToken = ref('');
        const currentFlight = ref(null);
        const scores = ref({}); // Map of playerID -> hole -> strokes

        // Player Form State
        const playerForm = ref({ id: 0, name: '', surname: '', reg_num: '', handicap: 0 });
        const isEditing = ref(false);

        // Fetch Players
        const fetchPlayers = async () => {
            const res = await fetch('/api/players');
            players.value = await res.json();
        };

        // Fetch Course
        const fetchCourse = async () => {
            const res = await fetch('/api/course');
            course.value = await res.json();
        };

        // Save Course
        const saveCourse = async () => {
            await fetch('/api/course', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(course.value)
            });
            alert('Course saved!');
        };

        // Fetch Flights
        const fetchFlights = async () => {
            const res = await fetch('/api/flights');
            flights.value = await res.json();
            setupDragAndDrop();
        };

        // Fetch Results
        const fetchResults = async () => {
            const res = await fetch('/api/results');
            results.value = await res.json();
        };

        // Upload Players
        const uploadPlayers = async (event) => {
            const file = event.target.files[0];
            if (!file) return;
            const formData = new FormData();
            formData.append('file', file);
            await fetch('/api/players/import', {
                method: 'POST',
                body: formData
            });
            fetchPlayers();
        };

        // Save Player (Create or Update)
        const savePlayer = async () => {
            if (!playerForm.value.name || !playerForm.value.surname) {
                alert('Name and Surname are required');
                return;
            }
            await fetch('/api/players', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(playerForm.value)
            });
            cancelEdit();
            fetchPlayers();
        };

        const editPlayer = (player) => {
            playerForm.value = { ...player };
            isEditing.value = true;
        };

        const cancelEdit = () => {
            playerForm.value = { id: 0, name: '', surname: '', reg_num: '', handicap: 0 };
            isEditing.value = false;
        };

        // Delete Player
        const deletePlayer = async (id) => {
            if (!confirm('Are you sure?')) return;
            await fetch('/api/players/delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id })
            });
            fetchPlayers();
        };

        // Create Flight
        const createFlight = async () => {
            if (!newFlightName.value) return;
            await fetch('/api/flights', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name: newFlightName.value })
            });
            newFlightName.value = '';
            fetchFlights();
        };

        // Delete Flight
        const deleteFlight = async (id) => {
            // if (!confirm('Are you sure you want to delete this flight? Players will be unassigned.')) return;
            await fetch('/api/flights', {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id })
            });
            fetchFlights();
            fetchPlayers(); // To update unassigned list
        };

        // Computed Unassigned Players
        const unassignedPlayers = computed(() => {
            const assignedIds = new Set();
            if (flights.value) {
                flights.value.forEach(f => {
                    if (f.players) {
                        f.players.forEach(p => assignedIds.add(p.id));
                    }
                });
            }
            return players.value.filter(p => !assignedIds.has(p.id));
        });

        // Drag and Drop Setup
        const setupDragAndDrop = () => {
            // Use nextTick to ensure DOM is updated
            Vue.nextTick(() => {
                const unassignedEl = document.getElementById('unassigned-players');
                if (unassignedEl) {
                    // Destroy existing instance if any (not tracking them currently, but Sortable handles re-init gracefully usually, 
                    // but better to be safe if we were tracking. For now just new Sortable)
                    new Sortable(unassignedEl, {
                        group: 'players',
                        animation: 150,
                        onAdd: async (evt) => {
                            const playerId = parseInt(evt.item.getAttribute('data-id'));
                            await unassignPlayer(playerId);
                        }
                    });
                }

                if (flights.value) {
                    flights.value.forEach(f => {
                        const el = document.getElementById('flight-' + f.id);
                        if (el) {
                            new Sortable(el, {
                                group: 'players',
                                animation: 150,
                                onAdd: async (evt) => {
                                    const playerId = parseInt(evt.item.getAttribute('data-id'));
                                    const flightId = parseInt(evt.to.getAttribute('data-flight-id'));
                                    await assignPlayer(flightId, playerId);
                                }
                            });
                        }
                    });
                }
            });
        };

        const assignPlayer = async (flightId, playerId) => {
            await fetch('/api/flights/assign', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ flight_id: flightId, player_id: playerId })
            });
            // Refresh to ensure state is consistent
            fetchFlights();
            fetchPlayers();
        };

        const unassignPlayer = async (playerId) => {
            await fetch('/api/flights/unassign', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ player_id: playerId })
            });
            fetchFlights();
            fetchPlayers();
        };

        // Scoring Logic
        const loadFlight = async () => {
            // In a real app, we'd search by token. For now, let's just find the flight in the list
            // But the API doesn't support search by token yet. 
            // Let's just fetch all flights and filter client side for this prototype
            await fetchFlights();
            currentFlight.value = flights.value.find(f => f.token === flightToken.value);
            if (!currentFlight.value) {
                alert('Flight not found');
                return;
            }

            // Fetch scores for all players in flight
            for (const player of currentFlight.value.players) {
                const res = await fetch(`/api/scores?player_id=${player.id}`);
                const playerScores = await res.json();
                for (const [hole, strokes] of Object.entries(playerScores)) {
                    scores.value[`${player.id}-${hole}`] = strokes;
                }
            }
        };

        const getScore = (playerId, hole) => {
            return scores.value[`${playerId}-${hole}`] || '';
        };

        const submitScore = async (playerId, hole, strokes) => {
            scores.value[`${playerId}-${hole}`] = strokes;
            await fetch('/api/scores', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    player_id: playerId,
                    hole_number: hole,
                    strokes: parseInt(strokes)
                })
            });
        };

        // ... (inside setup)

        // Distances (Mock data based on image for first 9, repeated/invented for back 9)
        const distances = [
            271, 316, 412, 114, 432, 178, 458, 351, 331,
            271, 316, 412, 114, 432, 178, 458, 351, 331 // Repeat for now
        ];

        const getDistance = (hole) => {
            return distances[hole - 1] || 0;
        };


        // ... (inside setup)

        watch(adminTab, (newVal) => {
            if (newVal === 'flights') {
                // Fetch flights again to be sure we have latest data and then setup DnD
                fetchFlights();
            }
        });

        onMounted(() => {
            fetchPlayers();
            fetchFlights();
            fetchResults();
            fetchCourse();

            // Check URL params
            const urlParams = new URLSearchParams(window.location.search);
            const viewParam = urlParams.get('view');
            const tokenParam = urlParams.get('token');

            if (viewParam === 'scoring' && tokenParam) {
                view.value = 'scoring';
                flightToken.value = tokenParam;
                loadFlight();
            }
        });

        // Helper: Get Initials
        const getInitials = (name, surname) => {
            return (name.charAt(0) + surname.charAt(0)).toUpperCase();
        };

        // Picker State
        const showPicker = ref(false);
        const pickerPlayerId = ref(0);
        const pickerHole = ref(0);
        const pickerValue = ref(0);
        const pickerList = ref(null);

        const openPicker = (playerId, hole, currentValue) => {
            pickerPlayerId.value = playerId;
            pickerHole.value = hole;

            if (currentValue) {
                pickerValue.value = parseInt(currentValue);
            } else {
                // Default to Par if available
                if (course.value && course.value[hole - 1]) {
                    pickerValue.value = course.value[hole - 1].par;
                } else {
                    pickerValue.value = 4; // Fallback
                }
            }

            showPicker.value = true;

            // Scroll to value after render
            nextTick(() => {
                scrollToValue(pickerValue.value);
            });
        };

        const scrollToValue = (val) => {
            pickerValue.value = val;
            if (pickerList.value) {
                const itemHeight = 40; // Must match CSS
                pickerList.value.scrollTop = val * itemHeight;
            }
        };

        const onScroll = (e) => {
            const itemHeight = 40;
            const scrollTop = e.target.scrollTop;
            const index = Math.round(scrollTop / itemHeight);
            pickerValue.value = index;
        };

        const selectOrConfirm = (val) => {
            if (pickerValue.value === val) {
                confirmScore();
            } else {
                scrollToValue(val);
            }
        };

        const confirmScore = async () => {
            await submitScore(pickerPlayerId.value, pickerHole.value, pickerValue.value);
            showPicker.value = false;
        };

        const getPar = (hole) => {
            return (course.value && course.value[hole - 1]) ? course.value[hole - 1].par : 4;
        };

        const getScoreStyle = (score, hole) => {
            if (!score) return { backgroundColor: '#E6E6E6' }; // Default gray

            const par = getPar(hole);
            const diff = parseInt(score) - par;

            // Premium Golf Color Palette
            if (diff === 0) return { backgroundColor: '#FCF6A1' }; // Par - Soft Yellow
            if (diff === 1) return { backgroundColor: '#D3D1EB' }; // Bogey - Lavender
            if (diff === 2) return { backgroundColor: '#BBB0EB' }; // Double Bogey - Purple
            if (diff >= 3) return { backgroundColor: '#9A78DB' }; // Triple+ - Deep Purple
            if (diff === -1) return { backgroundColor: '#F5C5C4' }; // Birdie - Soft Red/Salmon
            if (diff <= -2) return { backgroundColor: '#F59391' }; // Eagle+ - Vibrant Salmon

            return { backgroundColor: '#E6E6E6' };
        };

        const getPlayerTotal = (playerId, startHole, endHole) => {
            let total = 0;
            for (let h = startHole; h <= endHole; h++) {
                const s = scores.value[`${playerId}-${h}`];
                if (s) total += parseInt(s);
            }
            return total;
        };

        const closePicker = () => {
            showPicker.value = false;
        };

        return {
            view,
            adminTab,
            players,
            flights,
            results,
            course,
            saveCourse,
            newFlightName,
            flightToken,
            currentFlight,
            unassignedPlayers,
            playerForm,
            isEditing,
            savePlayer,
            editPlayer,
            cancelEdit,
            uploadPlayers,
            deletePlayer,
            createFlight,
            deleteFlight,
            loadFlight,
            getScore,
            submitScore,
            getInitials,
            showPicker,
            pickerValue,
            pickerHole,
            pickerList,
            openPicker,
            scrollToValue,
            onScroll,
            selectOrConfirm,
            confirmScore,
            closePicker,
            getDistance,
            getPar,
            getPlayerTotal,
            getScoreStyle
        };
    }
}).mount('#app');
